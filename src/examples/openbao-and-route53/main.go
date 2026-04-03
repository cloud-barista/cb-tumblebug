// main.go — OpenBao + Route53 Example (SDK Version)
//
// Workflow:
//  1. Load VAULT_ADDR, VAULT_TOKEN, ROUTE53_DOMAIN from .env
//  2. Fetch AWS credentials from OpenBao using OpenBao Go SDK (api/v2)
//  3. Use those credentials to query AWS Route53
//  4. Print results as a formatted table
//
// Run:
//	cp .env.example .env   # fill in VAULT_TOKEN and ROUTE53_DOMAIN
//	go mod tidy
//	go run main.go

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"bufio"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/openbao/openbao/api/v2"
)


// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	// 1. Load environment variables from .env
	loadEnv()

	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "http://localhost:8200"
	}
	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] VAULT_TOKEN is not set. Add it to .env or export it.")
		os.Exit(1)
	}
	domain := os.Getenv("ROUTE53_DOMAIN")
	if domain == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] ROUTE53_DOMAIN is not set. Add it to .env or export it.")
		os.Exit(1)
	}

	fmt.Printf("\n=== OpenBao + Route53 Example (SDK) ===\n")
	fmt.Printf("VAULT_ADDR    : %s\n", vaultAddr)
	fmt.Printf("ROUTE53_DOMAIN: %s\n\n", domain)

	// 2. Fetch AWS credentials from OpenBao
	awsCreds, err := fetchAWSCredsFromOpenBao(vaultAddr, vaultToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] OpenBao: %v\n", err)
		os.Exit(1)
	}

	// 3. Create Route53 client
	ctx := context.Background()
	r53, err := newRoute53Client(ctx, awsCreds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] AWS client: %v\n", err)
		os.Exit(1)
	}

	// 4. Find Hosted Zone
	fmt.Printf("[Route53] Searching hosted zone for: %s\n", domain)
	zoneID, zoneName, err := findHostedZone(ctx, r53, domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[Route53] Found zone: %s (%s)\n", zoneID, strings.TrimSuffix(zoneName, "."))

	// 5. List DNS records
	fmt.Printf("[Route53] Fetching DNS records...\n")
	records, err := listRecords(ctx, r53, zoneID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}

	// 6. Print results
	printRecords(domain, zoneID, zoneName, records)
}

// ── Step 1: Fetch AWS credentials from OpenBao ────────────────────────────────

type awsCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

// fetchAWSCredsFromOpenBao reads the secret at secret/data/csp/aws using the OpenBao SDK.
func fetchAWSCredsFromOpenBao(vaultAddr, vaultToken string) (*awsCredentials, error) {
	// Initialize OpenBao Client
	config := api.DefaultConfig()
	config.Address = vaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create OpenBao client: %w", err)
	}
	client.SetToken(vaultToken)

	// Read KV v2 secret with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secret, err := client.Logical().ReadWithContext(ctx, "secret/data/csp/aws")
	if err != nil {
		return nil, fmt.Errorf("read secret from OpenBao: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found at secret/data/csp/aws")
	}

	// Extract data (KV v2 returns data inside "data" field)
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid KV v2 secret format: 'data' field missing or not a map")
	}

	keyID, _ := data["AWS_ACCESS_KEY_ID"].(string)
	secretKey, _ := data["AWS_SECRET_ACCESS_KEY"].(string)

	if keyID == "" || secretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY is missing in secret")
	}

	region, _ := data["AWS_DEFAULT_REGION"].(string)
	if region == "" {
		region = "us-east-1"
	}

	fmt.Printf("[OpenBao] AWS credentials fetched using SDK\n")
	return &awsCredentials{
		AccessKeyID:     keyID,
		SecretAccessKey: secretKey,
		Region:          region,
	}, nil
}

// ── Step 2: Build AWS Route53 client ─────────────────────────────────────────

func newRoute53Client(ctx context.Context, creds *awsCredentials) (*route53.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				creds.AccessKeyID,
				creds.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build AWS config: %w", err)
	}
	return route53.NewFromConfig(cfg), nil
}

// ── Step 3: Find Hosted Zone ─────────────────────────────────────────────────

func findHostedZone(ctx context.Context, r53 *route53.Client, domain string) (zoneID, zoneName string, err error) {
	lookup := domain
	if !strings.HasSuffix(lookup, ".") {
		lookup += "."
	}

	out, err := r53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(lookup),
	})
	if err != nil {
		return "", "", fmt.Errorf("ListHostedZonesByName: %w", err)
	}

	for _, zone := range out.HostedZones {
		name := aws.ToString(zone.Name)
		if strings.HasSuffix(lookup, strings.TrimSuffix(name, ".")+".") || lookup == name {
			id := aws.ToString(zone.Id)
			return id, name, nil
		}
	}
	return "", "", fmt.Errorf("no hosted zone found for domain %q", domain)
}

// ── Step 4: List DNS records ──────────────────────────────────────────────────

type dnsRecord struct {
	Name    string
	Type    string
	TTL     int64
	Values  []string
}

func listRecords(ctx context.Context, r53 *route53.Client, zoneID string) ([]dnsRecord, error) {
	var records []dnsRecord
	var nextName *string
	var nextType types.RRType

	for {
		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId:    aws.String(zoneID),
			StartRecordName: nextName,
			// Only set StartRecordType if it's not empty, as types.RRType is essentially a string
			StartRecordType: types.RRType(nextType),
		}
		if string(nextType) == "" {
			input.StartRecordType = "" // empty string means no type filter
		}

		out, err := r53.ListResourceRecordSets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("ListResourceRecordSets: %w", err)
		}

		for _, rrs := range out.ResourceRecordSets {
			rec := dnsRecord{
				Name: aws.ToString(rrs.Name),
				Type: string(rrs.Type),
			}
			if rrs.TTL != nil {
				rec.TTL = *rrs.TTL
			}
			for _, r := range rrs.ResourceRecords {
				rec.Values = append(rec.Values, aws.ToString(r.Value))
			}
			if rrs.AliasTarget != nil {
				rec.Values = append(rec.Values,
					fmt.Sprintf("ALIAS → %s", aws.ToString(rrs.AliasTarget.DNSName)))
			}
			records = append(records, rec)
		}

		if !out.IsTruncated {
			break
		}
		nextName = out.NextRecordName
		nextType = out.NextRecordType
	}
	return records, nil
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func printRecords(domain, zoneID, zoneName string, records []dnsRecord) {
	fmt.Println()
	fmt.Printf("Domain      : %s\n", domain)
	fmt.Printf("Hosted Zone : %s  (%s)\n", zoneID, strings.TrimSuffix(zoneName, "."))
	fmt.Printf("Record count: %d\n", len(records))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tTTL\tVALUE")
	fmt.Fprintln(w, "────\t────\t───\t─────")

	for _, rec := range records {
		if len(rec.Values) == 0 {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", rec.Name, rec.Type, rec.TTL, "(none)")
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", rec.Name, rec.Type, rec.TTL, rec.Values[0])
		for _, v := range rec.Values[1:] {
			fmt.Fprintf(w, "\t\t\t%s\n", v)
		}
	}
	w.Flush()
	fmt.Println()
}

func loadEnv() {
	path := findEnvFile()
	if path == "" {
		fmt.Println("[INFO] No .env file found; relying on process environment.")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("[WARN] Could not load %s: %v\n", path, err)
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	fmt.Printf("[INFO] Loaded environment from: %s\n", path)
}

func findEnvFile() string {
	if _, err := os.Stat(".env"); err == nil {
		return ".env"
	}
	dir, _ := os.Getwd()
	for {
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			break
		}
		candidate := parent + "/.env"
		if _, err := os.Stat(candidate); err == nil {
			for _, marker := range []string{"go.mod", ".git", "Makefile"} {
				if _, err2 := os.Stat(parent + "/" + marker); err2 == nil {
					return candidate
				}
			}
		}
		dir = parent
	}
	return ""
}
