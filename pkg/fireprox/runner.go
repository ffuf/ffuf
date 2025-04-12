package fireprox

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// FireProxRunner implements the ffuf.RunnerProvider interface
// Exported for type assertions in main.go for cleanup
type FireProxRunner struct {
	simpleRunner   ffuf.RunnerProvider
	proxyManager   *ProxyManager
	config         *ffuf.Config
	proxyURL       string
	startupTested  bool
}

// NewFireProxRunner creates a new FireProx-enabled runner
func NewFireProxRunner(conf *ffuf.Config, replay bool, baseRunner ffuf.RunnerProvider) (ffuf.RunnerProvider, error) {
	// Create FireProx configuration
	fpConfig := &Config{
		AccessKey:    conf.FireProxAWSAccessKey,
		SecretKey:    conf.FireProxAWSSecretKey,
		SessionToken: conf.FireProxAWSSessionToken,
		Region:       conf.FireProxRegion,
		TargetURL:    conf.Url,
		NamePrefix:   "ffuf-fireprox",
		Debug:        conf.FireProxDebug,
	}

	// Validate AWS credentials before proceeding
	if err := ValidateAWSCredentials(fpConfig, conf.Context); err != nil {
		return nil, err
	}

	// Create proxy manager
	proxyManager, err := NewProxyManager(fpConfig, conf.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to create FireProx manager: %w", err)
	}

	// Use the provided base runner
	return &FireProxRunner{
		simpleRunner:  baseRunner,
		proxyManager:  proxyManager,
		config:        conf,
		proxyURL:      "",
		startupTested: false,
	}, nil
}

// Prepare passes the request preparation to the underlying runner
func (r *FireProxRunner) Prepare(input map[string][]byte, basereq *ffuf.Request) (ffuf.Request, error) {
	return r.simpleRunner.Prepare(input, basereq)
}

// Execute runs the request through FireProx
func (r *FireProxRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	// Create the initial proxy if this is the first request
	if !r.startupTested {
		err := r.setupAndTestProxy()
		if err != nil {
			return ffuf.Response{}, fmt.Errorf("failed to set up FireProx: %w", err)
		}
		r.startupTested = true
	}

	// Get the proxy URL
	proxyURL, err := r.proxyManager.GetNextProxy()
	if err != nil {
		return ffuf.Response{}, fmt.Errorf("failed to get FireProx proxy URL: %w", err)
	}
	r.proxyURL = proxyURL

	// Update the request's URL to use the proxy URL
	originalURL := req.Url
	
	// Preserve the path part of the URL by parsing the original URL
	// and combining it with the FireProx API Gateway URL
	proxyURLBase := proxyURL
	if proxyURLBase[len(proxyURLBase)-1] == '/' {
		proxyURLBase = proxyURLBase[:len(proxyURLBase)-1]
	}
	
	// Extract the path from the original URL
	path := ""
	if len(originalURL) > 0 {
		// Find where the path begins (after the third slash or after the host)
		pathStartIdx := len(originalURL)
		
		// Check if the URL starts with a scheme
		if len(originalURL) > 8 && (originalURL[:7] == "http://" || originalURL[:8] == "https://") {
			// Find the third slash which marks the beginning of the path
			doubleSlashIdx := -1
			if originalURL[:7] == "http://" {
				doubleSlashIdx = 6
			} else {
				doubleSlashIdx = 7
			}
			
			thirdSlashIdx := -1
			for i := doubleSlashIdx + 1; i < len(originalURL); i++ {
				if originalURL[i] == '/' {
					thirdSlashIdx = i
					break
				}
			}
			
			if thirdSlashIdx != -1 {
				pathStartIdx = thirdSlashIdx
			}
		}
		
		if pathStartIdx < len(originalURL) {
			path = originalURL[pathStartIdx:]
		}
	}
	
	// Combine the proxy URL with the path
	if path == "" {
		req.Url = proxyURL
	} else if path[0] == '/' {
		req.Url = proxyURLBase + path
	} else {
		req.Url = proxyURLBase + "/" + path
	}

	// Execute the request
	resp, err := r.simpleRunner.Execute(req)
	
	// Store both the original and proxy URLs in the response
	// For verbose output to display correctly
	if err == nil {
		// Store the proxy URL for verbose output
		resp.FireProxURL = req.Url
		
		// Restore the original URL in the response
		// This lets results display correctly with original FUZZ URL
		resp.Request.Url = originalURL
	}
	
	return resp, err
}

// Dump passes the dump operation to the underlying runner
func (r *FireProxRunner) Dump(req *ffuf.Request) ([]byte, error) {
	// Save original URL
	originalURL := req.Url
	
	// Get the proxy URL
	proxyURL, err := r.proxyManager.GetNextProxy()
	if err != nil {
		return nil, fmt.Errorf("failed to get FireProx proxy URL: %w", err)
	}
	
	// Preserve the path part of the URL by parsing the original URL
	// and combining it with the FireProx API Gateway URL
	proxyURLBase := proxyURL
	if proxyURLBase[len(proxyURLBase)-1] == '/' {
		proxyURLBase = proxyURLBase[:len(proxyURLBase)-1]
	}
	
	// Extract the path from the original URL
	path := ""
	if len(originalURL) > 0 {
		// Find where the path begins (after the third slash or after the host)
		pathStartIdx := len(originalURL)
		
		// Check if the URL starts with a scheme
		if len(originalURL) > 8 && (originalURL[:7] == "http://" || originalURL[:8] == "https://") {
			// Find the third slash which marks the beginning of the path
			doubleSlashIdx := -1
			if originalURL[:7] == "http://" {
				doubleSlashIdx = 6
			} else {
				doubleSlashIdx = 7
			}
			
			thirdSlashIdx := -1
			for i := doubleSlashIdx + 1; i < len(originalURL); i++ {
				if originalURL[i] == '/' {
					thirdSlashIdx = i
					break
				}
			}
			
			if thirdSlashIdx != -1 {
				pathStartIdx = thirdSlashIdx
			}
		}
		
		if pathStartIdx < len(originalURL) {
			path = originalURL[pathStartIdx:]
		}
	}
	
	// Combine the proxy URL with the path
	if path == "" {
		req.Url = proxyURL
	} else if path[0] == '/' {
		req.Url = proxyURLBase + path
	} else {
		req.Url = proxyURLBase + "/" + path
	}
	
	// Perform the dump
	dump, err := r.simpleRunner.Dump(req)
	
	// Restore the original URL in the request
	req.Url = originalURL
	
	return dump, err
}

// setupAndTestProxy initializes the FireProx API Gateway and tests connectivity
func (r *FireProxRunner) setupAndTestProxy() error {
	// Set up FireProx
	err := r.proxyManager.Setup()
	if err != nil {
		return fmt.Errorf("failed to set up FireProx: %w", err)
	}

	// Get the proxy URL
	proxyURL, err := r.proxyManager.GetNextProxy()
	if err != nil {
		return fmt.Errorf("failed to get FireProx proxy URL: %w", err)
	}
	r.proxyURL = proxyURL

	log.Printf("[FireProx] Created API Gateway proxy: %s", proxyURL)
	log.Printf("[FireProx] Testing connectivity through proxy...")

	// Test the proxy with a simple request
	testReq := ffuf.Request{
		Method:  "GET",
		Url:     proxyURL,
		Headers: make(map[string]string), // Initialize Headers map to prevent nil map panic
	}

	// Try the request with some retries as API Gateway deployment can take a few seconds
	var testResp ffuf.Response
	var testErr error
	maxRetries := 3
	retryDelay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		testResp, testErr = r.simpleRunner.Execute(&testReq)
		if testErr == nil {
			break
		}
		
		log.Printf("[FireProx] Test request failed (retry %d/%d): %v", i+1, maxRetries, testErr)
		if i < maxRetries-1 {
			log.Printf("[FireProx] Waiting %s before retry...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if testErr != nil {
		// Clean up the API Gateway if testing fails
		_ = r.proxyManager.Cleanup()
		return fmt.Errorf("failed to connect through FireProx after %d retries: %w", maxRetries, testErr)
	}

	log.Printf("[FireProx] Connectivity test successful, status code: %d", testResp.StatusCode)
	return nil
}

// Cleanup destroys all AWS API Gateway resources created by FireProx
func (r *FireProxRunner) Cleanup() error {
	if r.proxyManager == nil {
		return errors.New("proxy manager not initialized")
	}

	log.Printf("[FireProx] Cleaning up AWS resources...")
	err := r.proxyManager.Cleanup()
	if err != nil {
		return fmt.Errorf("error during FireProx cleanup: %w", err)
	}

	log.Printf("[FireProx] Cleanup completed successfully")
	return nil
}