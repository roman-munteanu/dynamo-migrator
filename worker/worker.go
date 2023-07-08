package worker

import (
	"context"
	"dynamo-migrator/m/v2/model"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Worker struct {
	Client      *dynamodb.Client
	TargetTable string
}

func (w Worker) Execute(ctx context.Context, req model.WorkRequest) error {
	var data []model.UserDetails

	for _, itemMap := range req.Items {
		var item model.UserDetails
		err := attributevalue.UnmarshalMap(itemMap, &item)
		if err != nil {
			return err
		}
		data = append(data, item)
	}
	// fmt.Println(data)

	// populate target
	for _, item := range data {
		userID := strconv.Itoa(item.UserID)

		_, err := w.Client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(w.TargetTable),
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberN{Value: userID},
				"name":    &types.AttributeValueMemberS{Value: item.Name},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
