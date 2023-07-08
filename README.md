dynamo-migrator
-----

This project does an asynchronous migration of data from one DynamoDB table to another.

## Build and Run

Build:
```
go mod tidy

go mod vendor
```

Run LocalStack:
```
docker-compose up -d

docker container ls
```

Run:
```
go run main.go
```


## DynamoDB Tables

original table:
```
aws dynamodb create-table \
    --table-name UsersOriginal \
    --attribute-definitions \
        AttributeName=user_id,AttributeType=N \
        AttributeName=platform,AttributeType=S \
    --key-schema \
        AttributeName=user_id,KeyType=HASH \
        AttributeName=platform,KeyType=RANGE \
    --provisioned-throughput \
        ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --endpoint-url=http://localhost:4566 
```

target table:
```
aws dynamodb create-table \
    --table-name UsersTarget \
    --attribute-definitions \
        AttributeName=user_id,AttributeType=N \
    --key-schema \
        AttributeName=user_id,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --endpoint-url=http://localhost:4566 
```


list:
```
aws dynamodb list-tables --endpoint-url=http://localhost:4566 
```


populate original table:
```
aws dynamodb batch-write-item \
    --request-items file://data-test.json \
    --return-consumed-capacity INDEXES \
    --return-item-collection-metrics SIZE \
    --endpoint-url=http://localhost:4566
```


or add each item:
```
aws dynamodb put-item \
    --table-name UsersOriginal \
    --item \
    '{"user_id": {"N": "101"}, "platform": {"S": "ios"}, "name": {"S": "Name 1"}}' \
    --endpoint-url=http://localhost:4566

aws dynamodb put-item \
    --table-name UsersOriginal \
    --item \
    '{"user_id": {"N": "102"}, "platform": {"S": "android"}, "name": {"S": "Name 2"}}' \
    --endpoint-url=http://localhost:4566

aws dynamodb put-item \
    --table-name UsersOriginal \
    --item \
    '{"user_id": {"N": "103"}, "platform": {"S": "ios"}, "name": {"S": "Name 3"}}' \
    --endpoint-url=http://localhost:4566
```

scan:
```
aws dynamodb scan --table-name UsersOriginal --endpoint-url=http://localhost:4566

aws dynamodb scan --table-name UsersTarget --endpoint-url=http://localhost:4566
```

query - is possible if GSI is created with `platform` as a HASH key:
```
aws dynamodb query --table-name UsersOriginal \
    --key-condition-expression "platform = :platform" \
    --expression-attribute-values '{":platform":{"S":"ios"}}' \
    --endpoint-url=http://localhost:4566
```

scan with filter:
```
aws dynamodb scan \
    --table-name UsersOriginal \
    --filter-expression "platform = :platform" \
    --expression-attribute-values '{":platform": {"S":"ios"}}' \
    --endpoint-url=http://localhost:4566
```

delete table:
```
aws dynamodb delete-table --table-name UsersOriginal --endpoint-url=http://localhost:4566 

aws dynamodb delete-table --table-name UsersTarget --endpoint-url=http://localhost:4566 
```

count:
```
aws dynamodb scan --table-name UsersOriginal --select COUNT --endpoint-url=http://localhost:4566

aws dynamodb scan --table-name UsersTarget --select COUNT --endpoint-url=http://localhost:4566
```
