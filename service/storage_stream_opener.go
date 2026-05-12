package service

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type StorageStreamOpener interface {
	Open(ctx context.Context, storageKey string) (io.ReadCloser, error)
}

type storageStreamOpener struct {
	ossDirect    *OSSDirectService
	uploadClient UploadServiceClient
}

func NewStorageStreamOpener(ossDirect *OSSDirectService, uploadClient UploadServiceClient) StorageStreamOpener {
	return &storageStreamOpener{
		ossDirect:    ossDirect,
		uploadClient: uploadClient,
	}
}

func (o *storageStreamOpener) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil, fmt.Errorf("storage_key is required")
	}
	if o.ossDirect != nil && o.ossDirect.Enabled() {
		return o.ossDirect.OpenObject(ctx, storageKey)
	}
	if o.uploadClient == nil {
		return nil, fmt.Errorf("upload service stream opener is not configured")
	}
	return o.uploadClient.OpenStoredFile(ctx, RemoteProbeStoredFileRequest{StorageKey: storageKey})
}
