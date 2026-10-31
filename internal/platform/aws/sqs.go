package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stayhub/backend/internal/config"
)

// NewSQSClient instantiates an AWS SQS client configured from the application environment.
func NewSQSClient(ctx context.Context, cfg *config.Config) (*sqs.Client, error) {
	var optFns []func(*awsconfig.LoadOptions) error
	if cfg.AWSRegion != "" {
		optFns = append(optFns, awsconfig.WithRegion(cfg.AWSRegion))
	}
	if cfg.S3AccessKey != "" && cfg.S3SecretKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for SQS: %w", err)
	}

	client := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.AWSEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
		}
	})

	return client, nil
}
