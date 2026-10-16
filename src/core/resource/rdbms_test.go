package resource

import (
	"errors"
	"strings"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/spf13/viper"
)

func TestValidateRDBMSCreateRequest_Capabilities(t *testing.T) {
	meta := spiderRDBMSMetaInfo{
		DBEngine:                         "mariadb",
		SupportedVersions:                []string{"10.4", "10.5", "10.6"},
		DBSpecOptions:                    []string{"mariadb.n2.medium.2c", "mariadb.n2.large.2c"},
		StorageTypeOptions:               []string{"cloud_essd", "cloud_auto"},
		SupportsStorageTypeSelection:     true,
		SupportsStorageSizeConfiguration: true,
		StorageSizeRangeGB: spiderStorageSizeRange{
			Min: 20,
			Max: 32768,
		},
	}

	t.Run("Valid spec matching DBSpecOptions", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "10.6",
			DBInstanceSpec:  "mariadb.n2.medium.2c",
			StorageType:     "cloud_essd",
			StorageSize:     20,
		}
		if err := validateRDBMSCreateRequest(meta, req, "alibaba"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid spec rejected", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "10.6",
			DBInstanceSpec:  "rds.mariadb.s1.small",
			StorageType:     "cloud_essd",
			StorageSize:     20,
		}
		err := validateRDBMSCreateRequest(meta, req, "alibaba")
		if err == nil {
			t.Fatal("expected error for invalid dbInstanceSpec, got nil")
		}
		expectedMsg := "dbInstanceSpec 'rds.mariadb.s1.small' is not supported for alibaba"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("Invalid engine version rejected", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "5.7",
			DBInstanceSpec:  "mariadb.n2.medium.2c",
			StorageType:     "cloud_essd",
			StorageSize:     20,
		}
		err := validateRDBMSCreateRequest(meta, req, "alibaba")
		if err == nil {
			t.Fatal("expected error for invalid engine version, got nil")
		}
		expectedMsg := "dbEngineVersion '5.7' is not supported for alibaba"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("Invalid storage type rejected", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "10.6",
			DBInstanceSpec:  "mariadb.n2.medium.2c",
			StorageType:     "invalid_ssd",
			StorageSize:     20,
		}
		err := validateRDBMSCreateRequest(meta, req, "alibaba")
		if err == nil {
			t.Fatal("expected error for invalid storage type, got nil")
		}
		expectedMsg := "storageType 'invalid_ssd' is not supported for alibaba"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("Storage size out of range rejected", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "10.6",
			DBInstanceSpec:  "mariadb.n2.medium.2c",
			StorageType:     "cloud_essd",
			StorageSize:     10,
		}
		err := validateRDBMSCreateRequest(meta, req, "alibaba")
		if err == nil {
			t.Fatal("expected error for storage size out of range, got nil")
		}
		expectedMsg := "storageSize 10 out of range [20-32768]"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("Public access rejected when not supported", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:        "mariadb",
			DBEngineVersion: "10.6",
			DBInstanceSpec:  "mariadb.n2.medium.2c",
			PublicAccess:    true,
		}
		// meta has SupportsPublicAccess: false
		err := validateRDBMSCreateRequest(meta, req, "ncp")
		if err == nil {
			t.Fatal("expected error for publicAccess=true on unsupported CSP, got nil")
		}
		expectedMsg := "publicAccess=true is not supported via API during automated provisioning for ncp"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})
}

func TestZeroCredentialPolicy(t *testing.T) {
	// Verify that RDBMSInfo contains no master credential fields and updates cleanly from Spider.
	var rdbmsInfo model.RDBMSInfo
	sp := spiderRDBMSInfo{
		DBEngine:       "mysql",
		MasterUserName: "",
		StorageSize:    "100",
		Status:         model.StorageStatusAvailable,
	}

	updateRDBMSInfoFromSpider(&rdbmsInfo, sp)
	if rdbmsInfo.StorageSize != 100 {
		t.Errorf("expected StorageSize 100, got %d", rdbmsInfo.StorageSize)
	}
	if rdbmsInfo.Status != model.StorageStatusAvailable {
		t.Errorf("expected Status %s, got %s", model.StorageStatusAvailable, rdbmsInfo.Status)
	}
}

func TestStorageSizeRangeGBMapping(t *testing.T) {
	// Verify that DataSource mapping translates StorageSizeRangeGB to storageSizeRangeGB.
	key := translateRDBMSDataSourceKey("StorageSizeRangeGB")
	if key != "storageSizeRangeGB" {
		t.Errorf("expected 'storageSizeRangeGB', got %q", key)
	}

	keyMin := translateRDBMSDataSourceKey("StorageSizeRangeGB.Min")
	if keyMin != "storageSizeRangeGB.min" {
		t.Errorf("expected 'storageSizeRangeGB.min', got %q", keyMin)
	}

	keyMax := translateRDBMSDataSourceKey("StorageSizeRangeGB.Max")
	if keyMax != "storageSizeRangeGB.max" {
		t.Errorf("expected 'storageSizeRangeGB.max', got %q", keyMax)
	}
}

func TestLoadRDBMSInfoYAML(t *testing.T) {
	// Verify that assets/rdbmsinfo.yaml unmarshals cleanly into model.RDBMSInfoConfig.
	var cfg model.RDBMSInfoConfig
	v := viper.New()
	v.SetConfigFile("../../../assets/rdbmsinfo.yaml")
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("failed to read rdbmsinfo.yaml: %v", err)
	}

	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("failed to unmarshal rdbmsinfo.yaml: %v", err)
	}

	awsCfg, ok := cfg.DBMS["aws"]
	if !ok {
		t.Fatal("expected 'aws' entry in rdbmsinfo.yaml DBMS map")
	}
	gp2, ok := awsCfg.StorageTypes["gp2"]
	if !ok {
		t.Fatal("expected 'gp2' entry under AWS storageTypes")
	}
	if gp2.MinStorageSizeGB != 20 {
		t.Errorf("expected AWS gp2 MinStorageSizeGB 20, got %d", gp2.MinStorageSizeGB)
	}
	if gp2.MaxStorageSizeGB != 70369 {
		t.Errorf("expected AWS gp2 MaxStorageSizeGB 70369, got %d", gp2.MaxStorageSizeGB)
	}

	gcpCfg, ok := cfg.DBMS["gcp"]
	if !ok {
		t.Fatal("expected 'gcp' entry in rdbmsinfo.yaml DBMS map")
	}
	pdSsd, ok := gcpCfg.StorageTypes["pd_ssd"]
	if !ok {
		t.Fatal("expected 'pd_ssd' entry under GCP storageTypes")
	}
	if pdSsd.MinStorageSizeGB != 10 {
		t.Errorf("expected GCP pd_ssd MinStorageSizeGB 10, got %d", pdSsd.MinStorageSizeGB)
	}
	if pdSsd.MaxStorageSizeGB != 70369 {
		t.Errorf("expected GCP pd_ssd MaxStorageSizeGB 70369, got %d", pdSsd.MaxStorageSizeGB)
	}

	mysqlReq, ok := awsCfg.DBMSRequirements["mysql"]
	if !ok {
		t.Fatal("expected 'mysql' entry under AWS dbmsRequirements")
	}
	if mysqlReq.MinStorageSizeGB != 20 {
		t.Errorf("expected AWS mysql MinStorageSizeGB 20, got %d", mysqlReq.MinStorageSizeGB)
	}
	if mysqlReq.MaxStorageSizeGB != 70369 {
		t.Errorf("expected AWS mysql MaxStorageSizeGB 70369, got %d", mysqlReq.MaxStorageSizeGB)
	}

	gcpMysqlReq, ok := gcpCfg.DBMSRequirements["mysql"]
	if !ok {
		t.Fatal("expected 'mysql' entry under GCP dbmsRequirements")
	}
	if gcpMysqlReq.ReferenceDBInstanceSpec != "db-n1-standard-1" {
		t.Errorf("expected GCP mysql referenceDBInstanceSpec 'db-n1-standard-1', got %q", gcpMysqlReq.ReferenceDBInstanceSpec)
	}
}

func TestApplyRDBMSCreateDefaults_SpecFallback(t *testing.T) {
	// If reference spec is not in live DBSpecOptions, it should not be assigned as DBInstanceSpec.
	meta := spiderRDBMSMetaInfo{
		DBEngine:      "mysql",
		DBSpecOptions: []string{"db-n1-standard-1", "db-n1-standard-2"},
	}
	req := model.RDBMSCreateRequest{
		AutoFillDefaults: true,
		DBEngine:         "mysql",
	}
	// With runtime DBMS info initialized, apply defaults.
	applyRDBMSCreateDefaults(meta, &req, "gcp")
	if req.DBInstanceSpec != "db-n1-standard-1" && req.DBInstanceSpec != "" {
		t.Errorf("unexpected DBInstanceSpec %q (should match supported option or fall back)", req.DBInstanceSpec)
	}
}

func TestValidateRDBMSCreateRequest_NCPDBACGToAllowAllInbound(t *testing.T) {
	meta := spiderRDBMSMetaInfo{
		DBEngine: "mysql",
	}

	t.Run("Rejected for non-NCP provider", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:                  "mysql",
			NCPDBACGToAllowAllInbound: true,
		}
		err := validateRDBMSCreateRequest(meta, req, "aws")
		if err == nil {
			t.Fatal("expected error when ncpDBACGToAllowAllInbound is used with aws, got nil")
		}
		expectedMsg := "ncpDBACGToAllowAllInbound is only supported for NCP"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("error %q does not contain expected %q", err.Error(), expectedMsg)
		}
	})

	t.Run("Accepted for NCP provider", func(t *testing.T) {
		req := model.RDBMSCreateRequest{
			DBEngine:                  "mysql",
			NCPDBACGToAllowAllInbound: true,
		}
		if err := validateRDBMSCreateRequest(meta, req, "ncp"); err != nil {
			t.Fatalf("unexpected error for NCP: %v", err)
		}
	})
}

func TestValidateRDBMSDatabaseCreateRequest(t *testing.T) {
	t.Run("Valid MySQL with charset and collation", func(t *testing.T) {
		req := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sample_db-1",
			Charset:      "utf8mb4",
			Collation:    "utf8mb4_unicode_ci",
		}
		if err := validateRDBMSDatabaseCreateRequest("mysql", "aws", req, "admin", "Password123!"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid databaseName characters rejected", func(t *testing.T) {
		req := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sample; DROP TABLE",
		}
		err := validateRDBMSDatabaseCreateRequest("mysql", "gcp", req, "", "")
		if err == nil || !errors.Is(err, ErrRDBMSDatabaseValidation) {
			t.Fatalf("expected ErrRDBMSDatabaseValidation for invalid databaseName, got: %v", err)
		}
	})

	t.Run("Invalid charset characters rejected", func(t *testing.T) {
		req := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8mb4-bad",
		}
		err := validateRDBMSDatabaseCreateRequest("mysql", "gcp", req, "", "")
		if err == nil || !errors.Is(err, ErrRDBMSDatabaseValidation) {
			t.Fatalf("expected ErrRDBMSDatabaseValidation for invalid charset, got: %v", err)
		}
	})

	t.Run("Invalid collation characters rejected", func(t *testing.T) {
		req := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Collation:    "utf8mb4_unicode_ci; --",
		}
		err := validateRDBMSDatabaseCreateRequest("mysql", "gcp", req, "", "")
		if err == nil || !errors.Is(err, ErrRDBMSDatabaseValidation) {
			t.Fatalf("expected ErrRDBMSDatabaseValidation for invalid collation, got: %v", err)
		}
	})

	t.Run("Charset rejected for PostgreSQL", func(t *testing.T) {
		req := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8",
		}
		err := validateRDBMSDatabaseCreateRequest("postgres", "aws", req, "admin", "Password123!")
		if err == nil || !errors.Is(err, ErrRDBMSDatabaseValidation) {
			t.Fatalf("expected ErrRDBMSDatabaseValidation for postgres charset, got: %v", err)
		}
	})

	t.Run("Alibaba MariaDB rejects collation but accepts charset", func(t *testing.T) {
		charsetOnly := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8mb4",
		}
		if err := validateRDBMSDatabaseCreateRequest("mariadb", "alibaba", charsetOnly, "", ""); err != nil {
			t.Fatalf("expected charset-only on Alibaba MariaDB to succeed, got: %v", err)
		}

		withCollation := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8mb4",
			Collation:    "utf8mb4_unicode_ci",
		}
		err := validateRDBMSDatabaseCreateRequest("mariadb", "alibaba", withCollation, "", "")
		if err == nil || !errors.Is(err, ErrRDBMSDatabaseValidation) {
			t.Fatalf("expected ErrRDBMSDatabaseValidation for Alibaba MariaDB collation, got: %v", err)
		}
	})

	t.Run("CSP DB admin credential requirements enforced", func(t *testing.T) {
		withOpts := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8mb4",
			Collation:    "utf8mb4_unicode_ci",
		}
		// NCP requires DB admin credentials when charset/collation is specified.
		if err := validateRDBMSDatabaseCreateRequest("mysql", "ncp", withOpts, "", ""); err == nil {
			t.Fatal("expected DB admin credential validation error for NCP with charset/collation")
		}
		if err := validateRDBMSDatabaseCreateRequest("mysql", "ncp", withOpts, "dbadmin", "Password123!"); err != nil {
			t.Fatalf("unexpected error for NCP with DB admin credentials: %v", err)
		}
		// Azure requires DB admin credentials when charset is specified without collation.
		charsetOnly := model.RDBMSDatabaseCreateReq{
			DatabaseName: "sampledb",
			Charset:      "utf8mb4",
		}
		if err := validateRDBMSDatabaseCreateRequest("mysql", "azure", charsetOnly, "", ""); err == nil {
			t.Fatal("expected DB admin credential validation error for Azure with charset-only")
		}
		if err := validateRDBMSDatabaseCreateRequest("mysql", "azure", withOpts, "", ""); err != nil {
			t.Fatalf("expected Azure with both charset and collation to not require DB admin credentials, got: %v", err)
		}
		// Tencent requires DB admin credentials only when collation is specified.
		if err := validateRDBMSDatabaseCreateRequest("mysql", "tencent", charsetOnly, "", ""); err != nil {
			t.Fatalf("expected Tencent with charset-only to not require DB admin credentials, got: %v", err)
		}
		if err := validateRDBMSDatabaseCreateRequest("mysql", "tencent", withOpts, "", ""); err == nil {
			t.Fatal("expected DB admin credential validation error for Tencent with collation")
		}
	})
}

func TestEnrichRDBMSDatabaseCreateError(t *testing.T) {
	rawErr := errors.New("dial tcp 10.7.1.6:3306: i/o timeout")

	ncpErr := enrichRDBMSDatabaseCreateError(rawErr, "ncp", "rdbms-ncp", "sampledb", "utf8mb4", "utf8mb4_unicode_ci")
	if !strings.Contains(ncpErr.Error(), "sys.ncp_create_db") {
		t.Errorf("expected NCP error enrichment to mention sys.ncp_create_db, got: %v", ncpErr)
	}

	nhnErr := enrichRDBMSDatabaseCreateError(rawErr, "nhn", "rdbms-nhn", "sampledb", "utf8mb4", "")
	if !strings.Contains(nhnErr.Error(), "Direct Control") {
		t.Errorf("expected NHN error enrichment to mention Direct Control, got: %v", nhnErr)
	}

	plainErr := enrichRDBMSDatabaseCreateError(rawErr, "ncp", "rdbms-ncp", "sampledb", "", "")
	if strings.Contains(plainErr.Error(), "sys.ncp_create_db") {
		t.Errorf("did not expect NCP charset hint when charset/collation are empty, got: %v", plainErr)
	}
}

