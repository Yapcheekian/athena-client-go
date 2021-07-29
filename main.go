package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go/aws"
)

var query = ``

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}

	client := athena.NewFromConfig(cfg)

	queryId, err := executeQuery(client, query)

	if err != nil {
		log.Fatalf("failed to execute query, %v", err)
	}

	status, err := getQueryExecutionState(client, queryId)

	if err != nil {
		log.Fatalf("failed to get execution state, %v", err)
	}

	for status != types.QueryExecutionStateSucceeded {
		newStatus, err := getQueryExecutionState(client, queryId)

		if err != nil {
			log.Fatalf("failed to get execution state, %v", err)
		}

		status = newStatus

		log.Println(status)

		time.Sleep(2 * time.Second)
	}

	result, err := getQueryResults(client, queryId)

	if err != nil {
		log.Fatalf("failed to stream result from S3, %v", err)
	}

	var rs [][]interface{}

	for i := range result.ResultSet.Rows {
		if i == 0 {
			continue
		}
		var temp []interface{}
		for j := range result.ResultSet.Rows[i].Data {
			temp = append(temp, *result.ResultSet.Rows[i].Data[j].VarCharValue)
		}
		rs = append(rs, temp)
	}

	fmt.Println(len(rs))
}

// getQueryResults stream result from Amazon S3
func getQueryResults(c *athena.Client, queryId *string) (*athena.GetQueryResultsOutput, error) {
	resultInput := &athena.GetQueryResultsInput{
		QueryExecutionId: queryId,
	}

	resp, err := c.GetQueryResults(context.TODO(), resultInput)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// getQueryExecutionState return the state of given queryId
func getQueryExecutionState(c *athena.Client, queryId *string) (types.QueryExecutionState, error) {
	getExecutionInput := &athena.GetQueryExecutionInput{
		QueryExecutionId: queryId,
	}

	// var status types.QueryExecutionState

	resp, err := c.GetQueryExecution(context.TODO(), getExecutionInput)

	if err != nil {
		return types.QueryExecutionStateFailed, err
	}

	return resp.QueryExecution.Status.State, nil

}

// executeQuery run the query and return query ID
func executeQuery(c *athena.Client, query string) (*string, error) {
	resultConf := types.ResultConfiguration{
		OutputLocation: aws.String("s3://cyberbiz-athena-output"),
	}

	executionInput := &athena.StartQueryExecutionInput{
		QueryString:         aws.String(query),
		ResultConfiguration: &resultConf,
	}

	resp, err := c.StartQueryExecution(context.TODO(), executionInput)

	if err != nil {
		return nil, err
	}

	return resp.QueryExecutionId, nil
}
