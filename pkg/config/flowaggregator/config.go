// Copyright 2021 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flowaggregator

type AggregatorTransportProtocol string

const (
	AggregatorTransportProtocolTCP AggregatorTransportProtocol = "TCP"
	AggregatorTransportProtocolTLS AggregatorTransportProtocol = "TLS"
	AggregatorTransportProtocolUDP AggregatorTransportProtocol = "UDP"
)

type AggregatorMode string

const (
	AggregatorModeAggregate AggregatorMode = "Aggregate"
	AggregatorModeProxy     AggregatorMode = "Proxy"
)

type FlowAggregatorConfig struct {
	Mode AggregatorMode `yaml:"mode,omitempty"`
	// Provide the active flow record timeout as a duration string. This determines
	// how often the flow aggregator exports the active flow records to the flow
	// collector. Thus, for flows with a continuous stream of packets, a flow record
	// will be exported to the collector once the elapsed time since the last export
	// event in the flow aggregator is equal to the value of this timeout.
	// Defaults to "60s". Valid time units are "ns", "us" (or "µs"), "ms", "s",
	// "m", "h".
	ActiveFlowRecordTimeout string `yaml:"activeFlowRecordTimeout,omitempty"`
	// Provide the inactive flow record timeout as a duration string. This determines
	// how often the flow aggregator exports the inactive flow records to the flow
	// collector. A flow record is considered to be inactive if no matching record
	// has been received by the flow aggregator in the specified interval.
	// Defaults to "90s". Valid time units are "ns", "us" (or "µs"), "ms", "s",
	// "m", "h".
	InactiveFlowRecordTimeout string `yaml:"inactiveFlowRecordTimeout,omitempty"`
	// Transport protocol over which the aggregator collects IPFIX records from all Agents.
	// Defaults to "tls"
	AggregatorTransportProtocol AggregatorTransportProtocol `yaml:"aggregatorTransportProtocol,omitempty"`
	// Provide an extra DNS name or IP address of flow aggregator for generating TLS certificate.
	FlowAggregatorAddress string `yaml:"flowAggregatorAddress,omitempty"`
	// RecordContents enables configuring some fields in the flow records. Fields can be
	// excluded to reduce record size.
	RecordContents RecordContentsConfig `yaml:"recordContents,omitempty"`
	// APIServer contains APIServer related configuration options.
	APIServer APIServerConfig `yaml:"apiServer,omitempty"`
	// Ignore records where the source or destination is in the FlowAggregatpr Namespace.
	// At the moment, it is only supported in Proxy mode.
	IgnoreFlowAggregatorNamespace bool `yaml:"ignoreFlowAggregatorNamespace,omitempty"`
	// FlowCollector contains external IPFIX or JSON collector related configuration options.
	FlowCollector FlowCollectorConfig `yaml:"flowCollector,omitempty"`
	// ClickHouse contains ClickHouse related configuration options.
	ClickHouse ClickHouseConfig `yaml:"clickHouse,omitempty"`
	// S3Uploader contains configuration options for uploading flow records to AWS S3.
	S3Uploader S3UploaderConfig `yaml:"s3Uploader,omitempty"`
	// FlowLogger contains configuration options for writing flow records to a local log file.
	FlowLogger FlowLoggerConfig `yaml:"flowLogger,omitempty"`
}

type RecordContentsConfig struct {
	PodLabels bool `yaml:"podLabels,omitempty"`
}

type APIServerConfig struct {
	// APIPort is the port for the antrea-agent APIServer to serve on.
	// Defaults to 10348.
	APIPort int `yaml:"apiPort,omitempty"`
	// Cipher suites to use.
	TLSCipherSuites string `yaml:"tlsCipherSuites,omitempty"`
	// TLS min version.
	TLSMinVersion string `yaml:"tlsMinVersion,omitempty"`
}

type FlowCollectorConfig struct {
	// Enable is the switch to enable exporting flow records to external flow collector.
	Enable bool `yaml:"enable,omitempty"`
	// Provide the flow collector address as string with format <IP>:<port>[:<proto>], where proto is tcp or udp.
	// If no L4 transport proto is given, we consider tcp as default.
	// Defaults to "".
	Address string `yaml:"address,omitempty"`
	// Provide the 32-bit Observation Domain ID which will uniquely identify this instance of the flow
	// aggregator to an external flow collector. If omitted, an Observation Domain ID will be generated
	// from the persistent cluster UUID generated by Antrea. Failing that (e.g. because the cluster UUID
	// is not available), a value will be randomly generated, which may vary across restarts of the flow
	// aggregator.
	ObservationDomainID *uint32 `yaml:"observationDomainID,omitempty"`
	// Provide format for records sent to the configured flow collector. Supported formats are IPFIX and JSON.
	// Defaults to "IPFIX"
	RecordFormat string `yaml:"recordFormat,omitempty"`
}

type ClickHouseConfig struct {
	// Enable is the switch to enable exporting flow records to ClickHouse.
	Enable bool `yaml:"enable,omitempty"`
	// Database is the name of database where Antrea "flows" table is created.
	Database string `yaml:"database,omitempty"`
	// DatabaseURL is the url to the database. Provide the database URL as a string with format
	// <Protocol>://<ClickHouse server FQDN or IP>:<ClickHouse port>. The protocol has to be one
	// from below: "tcp", "tls", "http", "https". When "tls" or "https" is used, tls will be enabled.
	// Defaults to "tcp://clickhouse-clickhouse.flow-visibility.svc:9000"
	DatabaseURL string `yaml:"databaseURL,omitempty"`
	// Debug enables debug logs from ClickHouse sql driver. Defaults to false.
	Debug bool `yaml:"debug,omitempty"`
	// Compress enables lz4 compression when committing flow records. Defaults to true.
	Compress *bool `yaml:"compress,omitempty"`
	// CommitInterval is the periodical interval between batch commit of flow records to DB.
	// Defaults to "8s". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	// Min value allowed is "1s".
	CommitInterval string `yaml:"commitInterval,omitempty"`
	// TLS configuration options, when using TLS to connect to the ClickHouse service.
	TLS TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
	// InsecureSkipVerify determines whether to skip the verification of the server's certificate chain and host name.
	// Default is false.
	InsecureSkipVerify bool `yaml:"insecureSkipVerify,omitempty"`
	// CACert determines whether to use custom CA certificate. Default root CAs will be used if false.
	// If true, a Secret named "flow-aggregator-ca" must be provided with the following keys:
	// ca.crt: <CA certificate>
	CACert bool `yaml:"caCert,omitempty"`
}

type S3UploaderConfig struct {
	// Enable is the switch to enable exporting flow records to AWS S3.
	// At the moment, the flow aggregator will look for the "standard" environment variables to
	// authenticate to AWS. These can be static credentials (AWS_ACCESS_KEY_ID,
	// AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN) or a Web Identity Token
	// (AWS_WEB_IDENTITY_TOKEN_FILE).
	Enable bool `yaml:"enable,omitempty"`
	// BucketName is the name of the S3 bucket to which flow records will be uploaded. If this
	// field is empty, initialization will fail.
	BucketName string `yaml:"bucketName"`
	// BucketPrefix is the prefix ("folder") under which flow records will be uploaded. If this
	// is omitted, flow records will be uploaded to the root of the bucket.
	BucketPrefix string `yaml:"bucketPrefix,omitempty"`
	// Region is used as a "hint" to get the region in which the provided bucket is located.
	// An error will occur if the bucket does not exist in the AWS partition the region hint
	// belongs to. If region is omitted, the value of the AWS_REGION environment variable will
	// be used, and if it is missing, we will default to "us-west-2".
	Region string `yaml:"region,omitempty"`
	// RecordFormat defines the format of the flow records uploaded to S3. Only "CSV" is
	// supported at the moment.
	RecordFormat string `yaml:"recordFormat,omitempty"`
	// Compress enables gzip compression when uploading files to S3. Defaults to true.
	Compress *bool `yaml:"compress,omitempty"`
	// MaxRecordsPerFile is the maximum number of records per file uploaded. It is not recommended
	// to change this value. Defaults to 1,000,000.
	MaxRecordsPerFile int32 `yaml:"maxRecordsPerFile,omitempty"`
	// UploadInterval is the duration between each file upload to S3.
	UploadInterval string `yaml:"uploadInterval,omitempty"`
}

type FlowLoggerConfig struct {
	// Enable is the switch to enable writing flow records to a local log file.
	Enable bool `yaml:"enable,omitempty"`
	// Path is the path to the local log file. Defaults to the antrea-flows.log file in the
	// operating system's default directory for temporary files (provided by os.TempDir).
	Path string `yaml:"path,omitempty"`
	// MaxSize is the maximum size in MB of a log file before it gets rotated. Defaults to 100MB.
	MaxSize int32 `yaml:"maxSize,omitempty"`
	// MaxBackups is the maximum number of old log files to retain. If set to 0, all log files
	// will be retained (unless MaxAge causes them to be deleted). Defaults to 3.
	MaxBackups int32 `yaml:"maxBackups,omitempty"`
	// MaxAge is the maximum number of days to retain old log files based on the timestamp
	// encoded in their filename. The default (0) is not to remove old log files based on age.
	MaxAge int32 `yaml:"maxAge,omitempty"`
	// Compress enables gzip compression on rotated files. Defaults to true.
	Compress *bool `yaml:"compress,omitempty"`
	// RecordFormat defines the format of the flow records logged to file. Only "CSV" is
	// supported at the moment.
	RecordFormat string `yaml:"recordFormat,omitempty"`
	// Filters can be used to select which flow records to log to file. The provided filters are
	// OR-ed to determine whether a specific flow should be logged. By default, all flows are
	// logged.
	Filters []FlowFilter `yaml:"filters,omitempty"`
	// PrettyPrint enables conversion of some numeric fields to a more meaningful string
	// representation.
	PrettyPrint *bool `yaml:"prettyPrint,omitempty"`
}

type NetworkPolicyRuleAction string

const (
	NetworkPolicyRuleActionNone   NetworkPolicyRuleAction = "None"
	NetworkPolicyRuleActionAllow  NetworkPolicyRuleAction = "Allow"
	NetworkPolicyRuleActionDrop   NetworkPolicyRuleAction = "Drop"
	NetworkPolicyRuleActionReject NetworkPolicyRuleAction = "Reject"
)

// FlowFilter will match a flow if all individual conditions are fulfilled.
type FlowFilter struct {
	// IngressNetworkPolicyRuleActions supports filtering based on the action name for the
	// ingress policy rule applied to the flow. By default, all actions are considered.
	IngressNetworkPolicyRuleActions []NetworkPolicyRuleAction `yaml:"ingressNetworkPolicyRuleActions,omitempty"`
	// EgressNetworkPolicyRuleActions supports filtering based on the action name for the egress
	// policy rule applied to the flow. By default, all actions are considered.
	EgressNetworkPolicyRuleActions []NetworkPolicyRuleAction `yaml:"egressNetworkPolicyRuleActions,omitempty"`
}
