package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	theRegion = "us-west-2"
)

type App struct {
	ctx           context.Context
	client        *dynamodb.Client
	originalTable string
	targetTable   string
}

type UserDetails struct {
	UserID   int    `dynamodbav:"user_id"`
	Platform string `dynamodbav:"platform"`
	Name     string `dynamodbav:"name"`
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
		ctx:           context.Background(),
		originalTable: "UsersOriginal",
		targetTable:   "UsersTarget",
	}
	app.init()

	platform := "ios"

	scanInput, err := app.scanInput(platform)
	if err != nil {
		panic(err)
	}

	var items []map[string]types.AttributeValue
	for {
		res, err := app.client.Scan(app.ctx, scanInput)
		if err != nil {
			panic(err)
		}

		items = append(items, res.Items...)

		// Scan limit of 1MB
		if res.LastEvaluatedKey == nil {
			break
		}

		scanInput.ExclusiveStartKey = res.LastEvaluatedKey
	}

	var data []UserDetails
	for _, itemMap := range items {
		var item UserDetails
		err = attributevalue.UnmarshalMap(itemMap, &item)
		if err != nil {
			panic(err)
		}
		data = append(data, item)
	}

	// fmt.Println(data)

	// populate target
	for _, item := range data {
		userID := strconv.Itoa(item.UserID)

		_, err = app.client.PutItem(app.ctx, &dynamodb.PutItemInput{
			TableName: aws.String(app.targetTable),
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberN{Value: userID},
				"name":    &types.AttributeValueMemberS{Value: item.Name},
			},
		})
		if err != nil {
			panic(err)
		}
	}

	app.scanTarget()
}

func (a *App) projection() expression.ProjectionBuilder {
	return expression.NamesList(
		expression.Name("user_id"),
		expression.Name("name"),
	)
}

func (a *App) scanInput(platform string) (*dynamodb.ScanInput, error) {
	filter := expression.Name("platform").Equal(expression.Value(platform))
	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(a.projection()).
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
	var items []UserDetails

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
