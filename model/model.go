package model

import "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

type UserDetails struct {
	UserID   int    `dynamodbav:"user_id"`
	Platform string `dynamodbav:"platform"`
	Name     string `dynamodbav:"name"`
}

type WorkRequest struct {
	Items []map[string]types.AttributeValue
}
