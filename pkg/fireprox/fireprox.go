package fireprox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Config holds the FireProx configuration
type Config struct {
	// AWS credentials
	AccessKey string
	SecretKey string
	SessionToken string
	
	// AWS region to create API Gateway in
	Region string
	
	// Target URL to proxy to
	TargetURL string
	
	// Name prefix for created API Gateways
	NamePrefix string
	
	// Debug mode
	Debug bool
}

// ProxyManager manages AWS API Gateway proxy
type ProxyManager struct {
	config     *Config
	client     *apigateway.Client
	proxy      *Proxy
	mu         sync.Mutex
	ctx        context.Context
	created    bool
}

// Proxy represents an API Gateway proxy instance
type Proxy struct {
	ID          string
	Name        string
	URL         string
	Region      string
	CreatedAt   time.Time
	Stage       string
	TargetURL   string
	RequestCount int
}

// ValidateAWSCredentials checks if the provided AWS credentials are valid using STS
func ValidateAWSCredentials(config *Config, ctx context.Context) error {
	if config.AccessKey == "" || config.SecretKey == "" {
		return fmt.Errorf("AWS access key and secret key are required")
	}
	
	// Clear environment variables first to prevent fallback
	origAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	origSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	origSessionToken := os.Getenv("AWS_SESSION_TOKEN")
	origProfile := os.Getenv("AWS_PROFILE")
	
	// Unset all AWS environment variables to ensure we only use provided credentials
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	os.Unsetenv("AWS_SESSION_TOKEN")
	os.Unsetenv("AWS_PROFILE")
	
	// Force use of the provided credentials only, don't allow fallback
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(config.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKey,
			config.SecretKey,
			config.SessionToken,
		)),
		// Disable shared config to prevent using ~/.aws/config
		awsconfig.WithSharedConfigProfile(""),
	}
	
	// Load AWS configuration
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		// Restore environment variables
		restoreEnvVars(origAccessKey, origSecretKey, origSessionToken, origProfile)
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	
	// Create STS client for caller identity check
	stsClient := sts.NewFromConfig(awsConfig)
	
	// Log that we're about to validate credentials
	log.Printf("[FireProx] Validating AWS credentials for Access Key ID: %s", maskString(config.AccessKey))
	
	// Use GetCallerIdentity which is specifically meant for credential validation
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	
	// Restore environment variables
	restoreEnvVars(origAccessKey, origSecretKey, origSessionToken, origProfile)
	
	if err != nil {
		// Log the detailed error for debugging 
		log.Printf("[FireProx] AWS credential validation error: %v", err)
		
		// Print error to stderr for user visibility
		fmt.Fprintf(os.Stderr, "AWS credential validation failed: %v\n", err)
		
		// Return specific error for caller
		return fmt.Errorf("AWS credential validation failed: %w", err)
	}
	
	// Log successful validation with account info
	log.Printf("[FireProx] AWS credentials validated successfully. Account ID: %s, User ARN: %s",
		aws.ToString(result.Account),
		aws.ToString(result.Arn))
	
	// Now verify API Gateway permissions
	apiClient := apigateway.NewFromConfig(awsConfig)
	log.Printf("[FireProx] Verifying API Gateway permissions...")
	
	_, err = apiClient.GetRestApis(ctx, &apigateway.GetRestApisInput{
		Limit: aws.Int32(1),
	})
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "AWS API Gateway permission check failed: %v\n", err)
		return fmt.Errorf("insufficient permissions for API Gateway: %w", err)
	}
	
	log.Printf("[FireProx] API Gateway permissions verified")
	return nil
}

// NewProxyManager creates a new FireProx proxy manager
func NewProxyManager(config *Config, ctx context.Context) (*ProxyManager, error) {
	// Set default values if not specified
	if config.NamePrefix == "" {
		config.NamePrefix = "ffuf-fireprox"
	}
	
	if config.Region == "" {
		return nil, errors.New("AWS region must be specified")
	}
	
	if config.TargetURL == "" {
		return nil, errors.New("target URL must be specified")
	}
	
	// Create AWS config
	var awsConfig aws.Config
	var err error
	
	// Configure AWS credentials
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(config.Region),
	}
	
	// Use explicit credentials if provided
	if config.AccessKey != "" && config.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKey,
			config.SecretKey,
			config.SessionToken,
		)))
	}
	
	// Load AWS configuration
	awsConfig, err = awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}
	
	// Create API Gateway client
	client := apigateway.NewFromConfig(awsConfig)
	
	return &ProxyManager{
		config:  config,
		client:  client,
		proxy:   nil,
		ctx:     ctx,
		created: false,
	}, nil
}

// Setup creates the API Gateway proxy
func (pm *ProxyManager) Setup() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.created {
		return nil
	}
	
	// Create proxy
	proxy, err := pm.createProxy()
	if err != nil {
		return err
	}
	
	pm.proxy = &proxy
	pm.created = true
	
	if pm.config.Debug {
		log.Printf("[FireProx] Created API Gateway proxy: %s", proxy.URL)
	}
	
	return nil
}

// createProxy creates a new API Gateway proxy
func (pm *ProxyManager) createProxy() (Proxy, error) {
	// Generate a unique name with timestamp
	timestamp := time.Now().Unix()
	name := fmt.Sprintf("%s-%d", pm.config.NamePrefix, timestamp)
	
	// Create REST API with OpenAPI definition
	template := generateOpenAPITemplate(pm.config.TargetURL)
	
	importInput := &apigateway.ImportRestApiInput{
		Body: []byte(template),
	}
	
	importResult, err := pm.client.ImportRestApi(pm.ctx, importInput)
	if err != nil {
		return Proxy{}, fmt.Errorf("failed to create API Gateway: %w", err)
	}
	
	// Deploy to a 'prod' stage
	prodStage := "prod"
	deployDescription := "ffuf FireProx deployment"
	deployInput := &apigateway.CreateDeploymentInput{
		RestApiId:    importResult.Id,
		StageName:    &prodStage,
		Description:  &deployDescription,
	}
	
	_, err = pm.client.CreateDeployment(pm.ctx, deployInput)
	if err != nil {
		// Try to clean up the created API on error
		_, cleanupErr := pm.client.DeleteRestApi(pm.ctx, &apigateway.DeleteRestApiInput{
			RestApiId: importResult.Id,
		})
		if cleanupErr != nil && pm.config.Debug {
			log.Printf("[FireProx] Failed to clean up API Gateway after deployment error: %v", cleanupErr)
		}
		return Proxy{}, fmt.Errorf("failed to deploy API Gateway: %w", err)
	}
	
	// Construct the proxy URL
	// Format: https://{api-id}.execute-api.{region}.amazonaws.com/{stage}/
	proxyURL := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/prod/", *importResult.Id, pm.config.Region)
	
	proxy := Proxy{
		ID:          *importResult.Id,
		Name:        name,
		URL:         proxyURL,
		Region:      pm.config.Region,
		CreatedAt:   time.Now(),
		Stage:       "prod",
		TargetURL:   pm.config.TargetURL,
		RequestCount: 0,
	}
	
	return proxy, nil
}

// GetNextProxy returns the proxy URL to use
func (pm *ProxyManager) GetNextProxy() (string, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// If no proxy has been created yet, create one
	if pm.proxy == nil {
		proxy, err := pm.createProxy()
		if err != nil {
			return "", err
		}
		pm.proxy = &proxy
		pm.created = true
		
		if pm.config.Debug {
			log.Printf("[FireProx] Created API Gateway proxy: %s", proxy.URL)
		}
	}
	
	// Increment request count (just for stats)
	pm.proxy.RequestCount++
	
	return pm.proxy.URL, nil
}

// GetProxy returns information about the created proxy
func (pm *ProxyManager) GetProxy() *Proxy {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	return pm.proxy
}

// TestProxy tests if the proxy is working by sending a request
func (pm *ProxyManager) TestProxy(proxyURL string) error {
	// This is handled by the Runner in ffuf
	return nil
}

// Cleanup deletes the created API Gateway proxy
func (pm *ProxyManager) Cleanup() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.proxy == nil {
		return nil
	}
	
	input := &apigateway.DeleteRestApiInput{
		RestApiId: &pm.proxy.ID,
	}
	
	_, err := pm.client.DeleteRestApi(pm.ctx, input)
	if err != nil {
		if pm.config.Debug {
			log.Printf("[FireProx] Failed to delete API Gateway %s: %v", pm.proxy.ID, err)
		}
		return fmt.Errorf("failed to delete API Gateway %s: %v", pm.proxy.ID, err)
	} 
	
	if pm.config.Debug {
		log.Printf("[FireProx] Successfully deleted API Gateway: %s", pm.proxy.ID)
	}
	
	// Clear the proxy
	pm.proxy = nil
	pm.created = false
	
	return nil
}

// maskString masks a string to display only the first 4 and last 4 characters
// This is useful for logging access keys without exposing sensitive information
func maskString(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

// restoreEnvVars restores AWS environment variables to their original values
func restoreEnvVars(accessKey, secretKey, sessionToken, profile string) {
	if accessKey != "" {
		os.Setenv("AWS_ACCESS_KEY_ID", accessKey)
	}
	if secretKey != "" {
		os.Setenv("AWS_SECRET_ACCESS_KEY", secretKey)
	}
	if sessionToken != "" {
		os.Setenv("AWS_SESSION_TOKEN", sessionToken)
	}
	if profile != "" {
		os.Setenv("AWS_PROFILE", profile)
	}
}

// generateOpenAPITemplate generates an OpenAPI template for creating an API Gateway proxy
func generateOpenAPITemplate(targetURL string) string {
	// Ensure the target URL doesn't end with a slash
	targetURL = strings.TrimRight(targetURL, "/")
	
	// Handle target URL containing FUZZ keyword
	proxyURI := targetURL + "/{proxy}"
	rootURI := targetURL + "/"
	
	// Check if the URL contains the FUZZ keyword and extract the base path
	if strings.Contains(targetURL, "FUZZ") {
		// When the URL contains FUZZ, we need to remove it for the API Gateway config
		// Get the host part of the URL (without FUZZ)
		var baseURL string
		
		if strings.HasPrefix(targetURL, "http://") {
			// Extract domain from http:// URL
			domainPart := targetURL[7:]
			slashPos := strings.Index(domainPart, "/")
			if slashPos > 0 {
				// URL has a path, get the domain only
				baseURL = "http://" + domainPart[:slashPos]
			} else {
				// URL is just a domain
				baseURL = targetURL
			}
		} else if strings.HasPrefix(targetURL, "https://") {
			// Extract domain from https:// URL
			domainPart := targetURL[8:]
			slashPos := strings.Index(domainPart, "/")
			if slashPos > 0 {
				// URL has a path, get the domain only
				baseURL = "https://" + domainPart[:slashPos]
			} else {
				// URL is just a domain
				baseURL = targetURL
			}
		} else {
			// No scheme, use as is
			baseURL = targetURL
		}
		
		// Remove trailing slash from baseURL if present
		baseURL = strings.TrimRight(baseURL, "/")
		
		// Set the proxy URI to point to the base URL (without FUZZ)
		proxyURI = baseURL + "/{proxy}"
		rootURI = baseURL + "/"
	}
	
	template := map[string]interface{}{
		"swagger": "2.0",
		"info": map[string]interface{}{
			"title":   "ffuf-fireprox",
			"version": "1.0",
		},
		"schemes": []string{"https"},
		"paths": map[string]interface{}{
			"/{proxy+}": map[string]interface{}{
				"x-amazon-apigateway-any-method": map[string]interface{}{
					"produces": []string{
						"application/json",
					},
					"parameters": []map[string]interface{}{
						{
							"name":     "proxy",
							"in":       "path",
							"required": true,
							"type":     "string",
						},
						{
							"name":     "X-My-X-Forwarded-For",
							"in":       "header",
							"required": false,
							"type":     "string",
						},
					},
					"responses": map[string]interface{}{},
					"x-amazon-apigateway-integration": map[string]interface{}{
						"uri":         proxyURI,
						"httpMethod":  "ANY",
						"type":        "http_proxy",
						"passthroughBehavior": "when_no_match",
						"requestParameters": map[string]string{
							"integration.request.path.proxy": "method.request.path.proxy",
							"integration.request.header.X-Forwarded-For": "method.request.header.X-My-X-Forwarded-For",
						},
					},
				},
			},
			"/": map[string]interface{}{
				"x-amazon-apigateway-any-method": map[string]interface{}{
					"produces": []string{
						"application/json",
					},
					"parameters": []map[string]interface{}{
						{
							"name":     "X-My-X-Forwarded-For",
							"in":       "header",
							"required": false,
							"type":     "string",
						},
					},
					"responses": map[string]interface{}{},
					"x-amazon-apigateway-integration": map[string]interface{}{
						"uri":         rootURI,
						"httpMethod":  "ANY",
						"type":        "http_proxy",
						"passthroughBehavior": "when_no_match",
						"requestParameters": map[string]string{
							"integration.request.header.X-Forwarded-For": "method.request.header.X-My-X-Forwarded-For",
						},
					},
				},
			},
		},
	}
	
	jsonBytes, err := json.Marshal(template)
	if err != nil {
		// Fallback to a simple string if marshaling fails
		return `{"swagger":"2.0","info":{"title":"ffuf-fireprox","version":"1.0"},"schemes":["https"],"paths":{"/{proxy+}":{"x-amazon-apigateway-any-method":{"produces":["application/json"],"parameters":[{"name":"proxy","in":"path","required":true,"type":"string"},{"name":"X-My-X-Forwarded-For","in":"header","required":false,"type":"string"}],"responses":{},"x-amazon-apigateway-integration":{"uri":"` + proxyURI + `","httpMethod":"ANY","type":"http_proxy","passthroughBehavior":"when_no_match","requestParameters":{"integration.request.path.proxy":"method.request.path.proxy","integration.request.header.X-Forwarded-For":"method.request.header.X-My-X-Forwarded-For"}}}}},"/":{"x-amazon-apigateway-any-method":{"produces":["application/json"],"parameters":[{"name":"X-My-X-Forwarded-For","in":"header","required":false,"type":"string"}],"responses":{},"x-amazon-apigateway-integration":{"uri":"` + rootURI + `","httpMethod":"ANY","type":"http_proxy","passthroughBehavior":"when_no_match","requestParameters":{"integration.request.header.X-Forwarded-For":"method.request.header.X-My-X-Forwarded-For"}}}}}}`
	}
	
	return string(jsonBytes)
}