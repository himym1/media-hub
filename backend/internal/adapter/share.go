package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"media-hub/backend/internal/search"
)

const ShareSourceID = "share"

type shareReference struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	ShareCode   string `json:"shareCode"`
	ReceiveCode string `json:"receiveCode,omitempty"`
}

// ShareImport receives a user-pasted 115 share into the adult workflow target.
type ShareImport struct {
	receiver ShareReceiver
}

func NewShareImport(receiver ShareReceiver) *ShareImport {
	return &ShareImport{receiver: receiver}
}

func (s *ShareImport) ID() string    { return ShareSourceID }
func (s *ShareImport) Label() string { return "115分享" }

func (s *ShareImport) Search(context.Context, string) ([]search.Candidate, error) {
	return nil, nil
}

func (s *ShareImport) StartTransfer(ctx context.Context, input search.TransferRequest) (search.TransferResult, error) {
	if strings.TrimSpace(input.DestinationID) == "" {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源选择无效", Retryable: false}
	}
	var reference shareReference
	if err := json.Unmarshal([]byte(input.Reference), &reference); err != nil {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	if reference.Kind != "share" || !frameHDRShareCode.MatchString(reference.ShareCode) || !frameHDRAccessCode.MatchString(reference.ReceiveCode) {
		return search.TransferResult{}, search.Failure{Code: "invalid_selection", Message: "资源引用无效", Retryable: false}
	}
	if s.receiver == nil {
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "115 分享接收器不可用", Retryable: false}
	}
	var videoNames []string
	if inspector, ok := s.receiver.(ShareInspector); ok {
		names, _, err := inspector.InspectShare(ctx, reference.ShareCode, reference.ReceiveCode)
		if err != nil {
			return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "115 分享打不开，已跳过这条资源", Retryable: automaticWriteRetryAllowed(err)}
		}
		videoNames = names
	}
	title := adultLibraryTitle(input.Title, append([]string{reference.Title}, videoNames...)...)
	destinationID, storageTitle, err := ensureTransferDestination(ctx, asFolderEnsurer(s.receiver), input.DestinationID, title, title)
	if err != nil {
		return search.TransferResult{}, transferFolderFailure(err)
	}
	if err := s.receiver.ReceiveShare(ctx, destinationID, reference.ShareCode, reference.ReceiveCode, nil); err != nil {
		var uncertain interface{ SubmissionUncertain() bool }
		if errors.As(err, &uncertain) && uncertain.SubmissionUncertain() {
			return search.TransferResult{}, search.Failure{Code: "source_submission_unknown", Message: "115 分享接收结果未知，需要人工确认", Retryable: true}
		}
		return search.TransferResult{}, search.Failure{Code: "source_unavailable", Message: "115 分享接收失败", Retryable: automaticWriteRetryAllowed(err)}
	}
	return search.TransferResult{OperationID: input.IdempotencyKey, Status: "completed", FileID: destinationID, Path: storageTitle, IsFile: false}, nil
}

func (s *ShareImport) TransferStatus(_ context.Context, _ int64, operationID string) (search.TransferResult, error) {
	return search.TransferResult{}, search.Failure{Code: "invalid_source_response", Message: fmt.Sprintf("115 分享转存没有可查询的异步状态: %s", operationID), Retryable: false}
}

func ShareReferenceJSON(title, shareCode, receiveCode string) (string, error) {
	payload, err := json.Marshal(shareReference{
		Kind: "share", Title: strings.TrimSpace(title), ShareCode: shareCode, ReceiveCode: receiveCode,
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
