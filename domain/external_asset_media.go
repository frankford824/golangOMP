package domain

import (
	"encoding/json"
	"strconv"
)

type ExternalMediaFingerprint struct {
	OriginHash            string          `json:"origin_path_hash"`
	Size                  int64           `json:"file_size"`
	ModifiedNS            int64           `json:"modified_ns"`
	ChangedNS             int64           `json:"changed_ns"`
	FileIdentity          string          `json:"file_identity"`
	RootIdentity          string          `json:"root_identity"`
	SourceVersion         string          `json:"source_version"`
	SHA256                string          `json:"sha256,omitempty"`
	ChecksumState         string          `json:"checksum_state"`
	OriginalSourceVersion string          `json:"original_source_version,omitempty"`
	PreviewSourceVersion  string          `json:"preview_source_version,omitempty"`
	AgentID               string          `json:"agent_id"`
	AgentEpoch            string          `json:"agent_epoch"`
	Sequence              int64           `json:"sequence"`
	ScanID                string          `json:"scan_id,omitempty"`
	Result                json.RawMessage `json:"media_result,omitempty"`
}

func (f *ExternalMediaFingerprint) Version(originPath string) string {
	return AssetMediaIdentity(f.RootIdentity, originPath, strconv.FormatInt(f.Size, 10),
		strconv.FormatInt(f.ModifiedNS, 10), strconv.FormatInt(f.ChangedNS, 10), f.FileIdentity)
}

type AssetMediaObject struct {
	Key           string `json:"key"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	CRC64         string `json:"crc64,omitempty"`
	SourceVersion string `json:"source_version"`
	Recipe        string `json:"recipe"`
}

type ExternalMediaResult struct {
	Original     *AssetMediaObject `json:"original,omitempty"`
	Preview      *AssetMediaObject `json:"preview,omitempty"`
	Thumbnail    *AssetMediaObject `json:"thumbnail,omitempty"`
	SourceSHA256 string            `json:"source_sha256,omitempty"`
	Package      *AssetMediaObject `json:"package,omitempty"`
}

type ExternalMediaScan struct {
	RootGeneration string `json:"-"`
	ScanID         string `json:"scan_id"`
	AgentID        string `json:"agent_id"`
	AgentEpoch     string `json:"agent_epoch"`
	RootIdentity   string `json:"root_identity"`
	OriginRoot     string `json:"origin_root"`
	StartSequence  int64  `json:"start_sequence"`
	EndSequence    int64  `json:"end_sequence"`
	ShardIndex     int    `json:"shard_index"`
	ShardCount     int    `json:"shard_count"`
	Parts          int    `json:"parts"`
	Files          int64  `json:"files"`
	ManifestSHA256 string `json:"manifest_sha256"`
	ReadErrors     int    `json:"read_errors"`
}

type ExternalMediaScanPart struct {
	ScanID string                         `json:"scan_id"`
	Part   int                            `json:"part"`
	SHA256 string                         `json:"sha256"`
	Items  []ExternalAssetFilesystemEvent `json:"items"`
}
