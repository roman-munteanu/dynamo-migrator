package main

import (
	"context"
	"dynamo-migrator/m/v2/model"
	"dynamo-migrator/m/v2/worker"
	"fmt"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	theRegion       = "us-west-2"
	numberOfWorkers = 4
	batchSize = 2
)

type App struct {
	ctx           context.Context
	client        *dynamodb.Client
	wg            sync.WaitGroup
	cancel        func()
	originalTable string
	targetTable   string
}

type workHandler interface {
	Execute(ctx context.Context, req model.WorkRequest) error
}

func (a *App) init() {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == dynamodb.ServiceID && region == theRegion {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           "http://localhost:4566",
				SigningRegion: theRegion,
			}, nil
		}

		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(a.ctx,
		config.WithRegion(theRegion),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("localstack", "localstack", "session")),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	a.client = dynamodb.NewFromConfig(cfg)
}

func main() {
	app := App{
		originalTable: "UsersOriginal",
		targetTable:   "UsersTarget",
	}
	app.ctx, app.cancel = context.WithCancel(context.Background())
	defer app.cancel()

	app.init()

	requestsCh := make(chan model.WorkRequest)

	var workers []worker.Worker
	for i := 0; i < numberOfWorkers; i++ {
		workers = append(workers, worker.Worker{
			Client:      app.client,
			TargetTable: app.targetTable,
		})
	}

	platform := "ios"

	items, err := app.readItems(platform)
	if err != nil {
		panic(err)
	}

	data := splitSlice(items, batchSize)
	var workRequests []model.WorkRequest
	for _, dataBatch := range data {
		workRequests = append(workRequests, model.WorkRequest{
			Items: dataBatch,
		})
	}

	go app.sendRequests(app.ctx, workRequests, requestsCh)

	for _, worker := range workers {
		app.wg.Add(1)
		go app.processRequests(app.ctx, worker, requestsCh)
	}

	app.wg.Wait()

	app.scanTarget()
}

func (a *App) readItems(platform string) ([]map[string]types.AttributeValue, error) {
	scanInput, err := a.scanInput(platform)
	if err != nil {
		return nil, err
	}

	var items []map[string]types.AttributeValue
	for {
		res, err := a.client.Scan(a.ctx, scanInput)
		if err != nil {
			return nil, err
		}

		items = append(items, res.Items...)

		// Scan limit of 1MB
		if res.LastEvaluatedKey == nil {
			break
		}
		scanInput.ExclusiveStartKey = res.LastEvaluatedKey
	}

	return items, nil
}

func (a *App) sendRequests(ctx context.Context, requests []model.WorkRequest, reqCh chan model.WorkRequest) {
	defer close(reqCh)

	for _, req := range requests {
		select {
		case <-ctx.Done():
			return
		case reqCh <- req:
		}
	}
}

func (a *App) processRequests(ctx context.Context, worker workHandler, reqCh chan model.WorkRequest) {
	defer a.wg.Done()

	for req := range reqCh {
		err := worker.Execute(ctx, req)
		if err != nil {
			log.Fatalln(err)
			a.cancel()
			return
		}
	}
}

func (a *App) scanInput(platform string) (*dynamodb.ScanInput, error) {
	filter := expression.Name("platform").Equal(expression.Value(platform))
	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(expression.NamesList(
			expression.Name("user_id"),
			expression.Name("name"),
		)).
		Build()
	if err != nil {
		return nil, err
	}

	return &dynamodb.ScanInput{
		TableName:                 aws.String(a.originalTable),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}, nil
}

func (a *App) scanTarget() {
	var items []model.UserDetails

	resp, err := a.client.Scan(a.ctx, &dynamodb.ScanInput{
		TableName: aws.String(a.targetTable),
	})
	if err != nil {
		panic(err)
	}

	err = attributevalue.UnmarshalListOfMaps(resp.Items, &items)
	if err != nil {
		panic(err)
	}

	fmt.Println(items)
}

func splitSlice[T any](s []T, size int) [][]T {
	var sl [][]T
	for len(s) > size {
		sl = append(sl, s[:size])
		s = s[size:]
	}
	if len(s) > 0 {
		sl = append(sl, s)
	}
	return sl
}
