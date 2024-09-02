package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/twisp/auth-go"
)

var (
	endpointFmt = "https://api.%s.cloud.twisp.com/financial/graphql/v1"
)

func handler(ctx context.Context, event cfn.Event) (string, map[string]any, error) {
	var physicalResourceID string
	switch event.ResourceType {
	case "Custom::TwispClientCreator":
		return handleClientCreation(ctx, event)
	default:
		return physicalResourceID, nil, fmt.Errorf("unknown resource type: %s", event.ResourceType)
	}
}

type ClientInput struct {
	Principal string         `json:"principal"`
	Name      string         `json:"name"`
	Policies  []ClientPolicy `json:"policies"`
}

type ClientPolicy struct {
	Effect     string            `json:"effect"`
	Actions    []string          `json:"actions"`
	Resources  []string          `json:"resources"`
	Assertions map[string]string `json:"assertions"`
}

func handleClientCreation(ctx context.Context, event cfn.Event) (string, map[string]any, error) {
	accountID, ok := event.ResourceProperties["AccountId"]
	if !ok {
		return "", nil, fmt.Errorf("missing AccountId")
	}
	region, ok := event.ResourceProperties["AccountId"]
	if !ok {
		region = os.Getenv("AWS_REGION")
	}

	endpoint := fmt.Sprintf(endpointFmt, region)
	twispHTTP := &http.Client{
		Transport: auth.NewRoundTripper(accountID.(string), region.(string), http.DefaultTransport),
		// Make sure lambda timeout is >= 5min
		Timeout: time.Second * 5,
	}

	switch event.RequestType {
	case cfn.RequestCreate:
		return handleCreate(twispHTTP, endpoint, event)
	case cfn.RequestUpdate:
		return handleUpdate(twispHTTP, endpoint, event)
	case cfn.RequestDelete:
		return handleDelete(twispHTTP, endpoint, event)
	default:
		return "", nil, fmt.Errorf("unknown request type: %s", event.RequestType)
	}
}

func handleCreate(twispHTTP *http.Client, endpoint string, event cfn.Event) (string, map[string]any, error) {
	client, ok := event.ResourceProperties["Client"]
	if !ok {
		return "", nil, fmt.Errorf("missing Client")
	}

	b, err := json.Marshal(client)
	if err != nil {
		return "", nil, err
	}

	var clientInput ClientInput
	if err := json.Unmarshal(b, &clientInput); err != nil {
		return "", nil, err
	}

	req := graphql.Request{
		Query: `mutation CreateClient($input: CreateClientInput!) {
	createClient(
		input: $input
	) {
		principal
	}
}`,
		Variables: map[string]any{
			"input": clientInput,
		},
		OpName: "CreateClient",
	}
	resp, err := doRequest(twispHTTP, req, endpoint)
	if err != nil {
		return "", nil, err
	}
	if len(resp.Errors) > 0 {
		return "", nil, resp.Errors[0].Err
	}
	return clientInput.Principal, nil, nil
}

func handleUpdate(twispHTTP *http.Client, endpoint string, event cfn.Event) (string, map[string]any, error) {
	client, ok := event.ResourceProperties["Client"]
	if !ok {
		return "", nil, fmt.Errorf("missing Client")
	}

	b, err := json.Marshal(client)
	if err != nil {
		return "", nil, err
	}

	var clientInput ClientInput
	if err := json.Unmarshal(b, &clientInput); err != nil {
		return "", nil, err
	}

	req := graphql.Request{
		Query: `mutation UpdateClient($principal:String!, $input: UpdateClientInput!) {
	auth {
		createClient(
    	principal: $principal,
			input: $input
		) {
			principal
		}
	}
}
`,
		Variables: map[string]any{
			"input":     clientInput,
			"principal": event.PhysicalResourceID,
		},
		OpName: "UpdateClient",
	}
	resp, err := doRequest(twispHTTP, req, endpoint)
	if err != nil {
		return "", nil, err
	}
	if len(resp.Errors) > 0 {
		return "", nil, resp.Errors[0].Err
	}
	return event.PhysicalResourceID, nil, nil
}

func handleDelete(twispHTTP *http.Client, endpoint string, event cfn.Event) (string, map[string]any, error) {
	req := graphql.Request{
		Query: `mutation DeleteClient($principal: String!) {
	auth {
		deleteClient(
    	principal: $principal,
		) {
			principal
		}
	}
}
`,
		Variables: map[string]any{
			"principal": event.PhysicalResourceID,
		},
		OpName: "DeleteCleint",
	}
	resp, err := doRequest(twispHTTP, req, endpoint)
	if err != nil {
		return "", nil, err
	}
	if len(resp.Errors) > 0 {
		return "", nil, resp.Errors[0].Err
	}
	return event.PhysicalResourceID, nil, nil
}

func doRequest(client *http.Client, req graphql.Request, endpoint string) (*graphql.Response, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	out, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var graphqlResp graphql.Response
	if err := json.Unmarshal(out, &graphqlResp); err != nil {
		return nil, err
	}

	return &graphqlResp, nil
}

func main() {
	lambda.Start(cfn.LambdaWrap(handler))
}
