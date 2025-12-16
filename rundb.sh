#!/bin/bash

docker run -d -p 8000:8000 --name dynamodb-local amazon/dynamodb-local

## import csv to dynamo
aws dynamodb create-table --table-name YourTableName --attribute-definitions AttributeName=id,AttributeType=N AttributeName=name,AttributeType=S --key-schema AttributeName=id,KeyType=HASH AttributeName=name,KeyType=RANGE --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 --endpoint-url http://localhost:8000