package adapter

import (
	"context"
	"encoding/json"
	"testing"

	"media-hub/backend/internal/search"
)

type shareReceiverStub struct {
	destination string
	share       string
	receive     string
	folderName  string
	videoNames  []string
	urls        []string
}

func (s *shareReceiverStub) ReceiveShare(_ context.Context, destinationID, shareCode, receiveCode string, _ []string) error {
	s.destination = destinationID
	s.share = shareCode
	s.receive = receiveCode
	return nil
}

func (s *shareReceiverStub) InspectShare(context.Context, string, string) ([]string, []string, error) {
	return s.videoNames, []string{"root"}, nil
}

func (s *shareReceiverStub) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	s.folderName = name
	return parentID + "-child", nil
}

func (s *shareReceiverStub) AddOfflineURLs(_ context.Context, destinationID string, urls []string) error {
	s.destination = destinationID
	s.urls = urls
	return nil
}

func TestShareImportReceivesAdultFolder(t *testing.T) {
	receiver := &shareReceiverStub{videoNames: []string{"SSIS-001.2160p.mkv", "sample.mp4"}}
	source := NewShareImport(receiver)
	reference, err := json.Marshal(shareReference{Kind: ImportKindShare, Title: "115分享 abc", ShareCode: "shareABC123", ReceiveCode: "ab12"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "115分享 shareABC123", MediaType: "adult", Reference: string(reference), DestinationID: "dest", IdempotencyKey: "share-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FileID != "dest-child" || result.Path != "SSIS-001" || receiver.folderName != "SSIS-001" || receiver.share != "shareABC123" {
		t.Fatalf("result=%#v folder=%q share=%q", result, receiver.folderName, receiver.share)
	}
}

func TestShareImportSubmitsOfflineURL(t *testing.T) {
	receiver := &shareReceiverStub{}
	source := NewShareImport(receiver)
	reference, err := URLReferenceJSON("SSIS-001", "https://cdn.example.com/SSIS-001.mkv")
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "SSIS-001", MediaType: "adult", Reference: reference, DestinationID: "dest", IdempotencyKey: "url-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pending" || result.FileID != "dest-child" || result.Path != "SSIS-001" || len(receiver.urls) != 1 {
		t.Fatalf("result=%#v urls=%q folder=%q", result, receiver.urls, receiver.folderName)
	}
}

func TestShareImportRejectsInvalidReference(t *testing.T) {
	source := NewShareImport(&shareReceiverStub{})
	if _, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "x", MediaType: "adult", Reference: "{}", DestinationID: "dest",
	}); err == nil {
		t.Fatal("expected invalid reference")
	}
}

func TestShareImportMarksUploadedFolderComplete(t *testing.T) {
	source := NewShareImport(&shareReceiverStub{})
	reference, err := UploadedReferenceJSON("SSIS-001", "9001")
	if err != nil {
		t.Fatal(err)
	}
	result, err := source.StartTransfer(context.Background(), search.TransferRequest{
		Title: "SSIS-001", MediaType: "adult", Reference: reference, DestinationID: "dest", IdempotencyKey: "up-op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.FileID != "9001" || result.Path != "SSIS-001" || result.IsFile {
		t.Fatalf("result=%#v", result)
	}
}
