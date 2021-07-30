package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go/aws"
)

var (
	query          string
	s3Bucket       string
	slackWebhook   string
	alertThreshold int
	startTime      string
	endTime        string
)

func init() {
	s3Bucket = os.Getenv("S3_BUCKET")
	slackWebhook = os.Getenv("SLACK_WEBHOOK")
	alertThreshold, _ = strconv.Atoi(os.Getenv("ALERT_THRESHOLD"))

	byteSlices, err := ioutil.ReadFile("./query.sql")

	if err != nil {
		log.Fatalf("failed to load sql file, %v", err)
	}

	query = string(byteSlices)
}

func main() {
	// default config is loaded from AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}

	client := athena.NewFromConfig(cfg)

	startTime = time.Now().Add(-time.Duration(5) * time.Minute).Format("2006-01-02 15:04:05")
	endTime = time.Now().Format("2006-01-02 15:04:05")
	queryId, err := executeQuery(client, query)

	if err != nil {
		fmt.Println(query)
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

	log.Printf("%v - %v : %d requests were denied access", startTime, endTime, len(rs))

	if len(rs) > alertThreshold && slackWebhook != "" {
		postBody, _ := json.Marshal(map[string]string{
			"text": fmt.Sprintf("%v - %v : %d requests were denied access", startTime, endTime, len(rs)),
		})

		responseBody := bytes.NewBuffer(postBody)

		_, err = http.Post(slackWebhook, "application/json", responseBody)

		if err != nil {
			log.Fatalf("failed to send slack webhook, %v", err)
		}
	}
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

	resp, err := c.GetQueryExecution(context.TODO(), getExecutionInput)

	if err != nil {
		return types.QueryExecutionStateFailed, err
	}

	return resp.QueryExecution.Status.State, nil

}

// executeQuery run the query and return query ID
func executeQuery(c *athena.Client, query string) (*string, error) {
	resultConf := types.ResultConfiguration{
		OutputLocation: aws.String(fmt.Sprintf("s3://%s", s3Bucket)),
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
