package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	serverconfig "github.com/us/den/internal/config"
	"github.com/us/den/internal/runtime"
)

// S3Client wraps the AWS S3 client with den-specific operations.
type S3Client struct {
	client *s3.Client
	logger *slog.Logger
}

// S3Credentials holds the resolved credentials for an S3 operation.
type S3Credentials struct {
	Endpoint  string
	Bucket    string
	Prefix    string
	Region    string
	AccessKey string
	SecretKey string
}

// ResolveS3Credentials resolves credentials from per-sandbox config
// falling back to server-wide defaults.
func ResolveS3Credentials(sandbox *runtime.S3SyncConfig, server serverconfig.S3Config) (*S3Credentials, error) {
	creds := &S3Credentials{}

	// Endpoint
	if sandbox != nil && sandbox.Endpoint != "" {
		creds.Endpoint = sandbox.Endpoint
	} else if server.Endpoint != "" {
		creds.Endpoint = server.Endpoint
	}

	// Bucket
	if sandbox != nil && sandbox.Bucket != "" {
		creds.Bucket = sandbox.Bucket
	}
	if creds.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	// Prefix
	if sandbox != nil {
		creds.Prefix = sandbox.Prefix
	}

	// Region
	if sandbox != nil && sandbox.Region != "" {
		creds.Region = sandbox.Region
	} else if server.Region != "" {
		creds.Region = server.Region
	} else {
		creds.Region = "us-east-1"
	}

	// Access key
	if sandbox != nil && sandbox.AccessKey != "" {
		creds.AccessKey = sandbox.AccessKey
	} else if server.AccessKey != "" {
		creds.AccessKey = server.AccessKey
	} else {
		return nil, fmt.Errorf("S3 access key is required")
	}

	// Secret key
	if sandbox != nil && sandbox.SecretKey != "" {
		creds.SecretKey = sandbox.SecretKey
	} else if server.SecretKey != "" {
		creds.SecretKey = server.SecretKey
	} else {
		return nil, fmt.Errorf("S3 secret key is required")
	}

	return creds, nil
}

// ssrfBlockingTransport returns an http.RoundTripper that blocks connections
// to private/internal IP addresses at the dial level, preventing DNS rebinding attacks.
func ssrfBlockingTransport() http.RoundTripper {
	dialer := &net.Dialer{}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}
			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve host: %w", err)
			}
			for _, rawIP := range ips {
				ip := net.ParseIP(rawIP)
				if ip == nil || isBlockedIP(ip) {
					return nil, fmt.Errorf("connection to %s blocked (resolves to internal address)", host)
				}
			}
			// Connect to the first resolved IP to prevent re-resolution
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
		},
	}
}

// isBlockedIP returns true if the IP is in a private, loopback, link-local,
// or otherwise internal range that should not be accessible via SSRF.
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		IsCloudMetadataIP(ip)
}

// Cloud metadata IP addresses (pre-parsed for efficiency).
var (
	metadataIP     = net.ParseIP("169.254.169.254") // AWS, GCP, Azure
	metadataAltIP  = net.ParseIP("169.254.169.253") // Azure alternative
	alibabaCloudIP = net.ParseIP("100.100.100.200") // Alibaba Cloud
)

// Cloud metadata IPv6 CIDR ranges.
var cloudMetadataV6Ranges []*net.IPNet

func init() {
	_, cidr, _ := net.ParseCIDR("fd00:ec2::/32") // AWS IPv6 metadata
	cloudMetadataV6Ranges = append(cloudMetadataV6Ranges, cidr)
}

// IsCloudMetadataIP checks for cloud provider metadata IP ranges.
func IsCloudMetadataIP(ip net.IP) bool {
	if ip.Equal(metadataIP) || ip.Equal(metadataAltIP) || ip.Equal(alibabaCloudIP) {
		return true
	}
	for _, cidr := range cloudMetadataV6Ranges {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// NewS3Client creates a new S3Client from resolved credentials.
// When a custom endpoint is provided, the client uses an SSRF-blocking transport
// that validates resolved IPs at dial time to prevent DNS rebinding attacks.
func NewS3Client(ctx context.Context, creds *S3Credentials, logger *slog.Logger) (*S3Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(creds.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(creds.AccessKey, creds.SecretKey, ""),
		),
	}

	// When using a custom endpoint, inject SSRF-blocking transport
	// to prevent DNS rebinding attacks (TOCTOU between validation and connection).
	if creds.Endpoint != "" {
		opts = append(opts, awsconfig.WithHTTPClient(
			&http.Client{Transport: ssrfBlockingTransport()},
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}
	if creds.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = &creds.Endpoint
			o.UsePathStyle = true // Required for MinIO
		})
	}

	client := s3.NewFromConfig(cfg, s3Opts...)
	return &S3Client{client: client, logger: logger}, nil
}

// Download downloads an object from S3 and returns its body.
func (c *S3Client) Download(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("downloading s3://%s/%s: %w", bucket, key, err)
	}
	size := int64(0)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}
	return resp.Body, size, nil
}

// Upload uploads data to S3.
func (c *S3Client) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   body,
	}
	if size > 0 {
		input.ContentLength = &size
	}
	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("uploading to s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

// ListObjects lists objects in a bucket with the given prefix.
// If maxKeys > 0, at most maxKeys objects are returned.
func (c *S3Client) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]string, error) {
	var keys []string
	input := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	}
	if maxKeys > 0 {
		mk := int32(min(maxKeys, 1000))
		input.MaxKeys = &mk
	}
	paginator := s3.NewListObjectsV2Paginator(c.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing s3://%s/%s: %w", bucket, prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
				if maxKeys > 0 && len(keys) >= maxKeys {
					return keys, nil
				}
			}
		}
	}
	return keys, nil
}
