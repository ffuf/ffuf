# FireProx Integration for ffuf

## Overview

FireProx integration allows ffuf to dynamically create and manage AWS API Gateway instances that act as proxies to target websites. This helps in bypassing IP-based rate limiting and blocking by rotating through multiple AWS-owned IP addresses during scanning.

## Features

- Creates and manages an AWS API Gateway proxy dynamically
- Automatic IP rotation with each request (built into API Gateway)
- Automatic cleanup of AWS resources on Ctrl+C interruption
- Cleanup of AWS resources after scan completion
- Testing of proxy connectivity before starting scans
- Support for multiple AWS regions

## Usage

```
ffuf -w /path/to/wordlist -u https://target-site.com/FUZZ -fireprox -fireprox-region us-east-1
```

AWS credentials can be supplied via environment variables (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`) or through command-line flags:

```
ffuf -w /path/to/wordlist -u https://target-site.com/FUZZ -fireprox -fireprox-region us-east-1 -fireprox-access-key YOUR_ACCESS_KEY -fireprox-secret-key YOUR_SECRET_KEY
```

## Options

The following options are available:

- `-fireprox`: Enable FireProx integration
- `-fireprox-region`: AWS Region for creating API Gateway proxy (default: us-east-1)
- `-fireprox-access-key`: AWS Access Key for FireProx (or use AWS_ACCESS_KEY_ID env var)
- `-fireprox-secret-key`: AWS Secret Key for FireProx (or use AWS_SECRET_ACCESS_KEY env var)
- `-fireprox-session-token`: AWS Session Token for FireProx (optional)
- `-fireprox-debug`: Enable verbose debug output for FireProx operations

## Configuration File Example

You can also configure FireProx in your ffufrc configuration file:

```
[fireprox]
    # Enable FireProx integration for IP rotation through AWS API Gateway
    enable = true
    # AWS Region for the API Gateway
    region = "us-east-1"
    # AWS Access Key ID (or use environment variable AWS_ACCESS_KEY_ID)
    access_key = "YOUR_AWS_ACCESS_KEY"
    # AWS Secret Access Key (or use environment variable AWS_SECRET_ACCESS_KEY)
    secret_key = "YOUR_AWS_SECRET_KEY"
    # AWS Session Token (optional)
    # session_token = ""
    # Enable debug output for FireProx operations
    debug = false
```

## Requirements

- AWS API Gateway permissions
- AWS credentials with appropriate permissions
- Supported AWS regions (most regions with API Gateway support)

## How It Works

1. When you run ffuf with the `-fireprox` flag, a new AWS API Gateway is created in the specified region.
2. The API Gateway is configured to proxy requests to the target website.
3. All fuzzing requests are sent through the API Gateway, which rotates IP addresses automatically.
4. When ffuf completes or is interrupted with Ctrl+C, the AWS resources are automatically cleaned up.

To see the actual AWS API Gateway URLs being used for your requests, run ffuf with the `-v` (verbose) flag. In verbose mode, each result will show both the original target URL and the AWS API Gateway URL that was used to make the request:

```
[Status: 200, Size: 4242, Words: 1337, Lines: 420, Duration: 189ms]
| URL | https://target-site.com/path
| AWS | https://ab12cd34ef.execute-api.us-east-1.amazonaws.com/prod/path
```

## IP Rotation

The IP rotation happens automatically through the AWS API Gateway. AWS maintains a pool of IP addresses for API Gateway instances, and these IPs are rotated naturally as requests are made. This helps to bypass rate limiting and IP-based blocking mechanisms.

## Troubleshooting

- If you see "The security token included in the request is invalid" errors, check your AWS credentials.
- Make sure your AWS user has sufficient permissions to create and manage API Gateway resources.
- If you need to manually clean up resources, you can use the AWS Console to delete API Gateway instances.
- Use the `-v` flag to verify that your requests are actually going through the AWS API Gateway.
- For more detailed logs, add the `-fireprox-debug` flag.
