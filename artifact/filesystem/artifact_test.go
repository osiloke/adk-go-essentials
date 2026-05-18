package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/adk/artifact"
	"google.golang.org/genai"
)

func TestFileSystemArtifactService_CRUD(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	svc := NewFileSystemArtifactService(baseDir)

	req := &artifact.SaveRequest{
		SessionID: "default",
		FileName:  "test.txt",
		Part: &genai.Part{
			InlineData: &genai.Blob{Data: []byte("hello artifact")},
		},
	}

	saveResp, err := svc.Save(ctx, req)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if saveResp == nil || saveResp.Version == 0 {
		t.Fatalf("unexpected save response: %#v", saveResp)
	}

	listResp, err := svc.List(ctx, &artifact.ListRequest{SessionID: "default"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listResp.FileNames) != 1 || listResp.FileNames[0] != "test.txt" {
		t.Fatalf("unexpected list response: %#v", listResp.FileNames)
	}

	loadResp, err := svc.Load(ctx, &artifact.LoadRequest{SessionID: "default", FileName: "test.txt"})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadResp == nil || loadResp.Part == nil || loadResp.Part.InlineData == nil {
		t.Fatalf("unexpected load response: %#v", loadResp)
	}
	if string(loadResp.Part.InlineData.Data) != "hello artifact" {
		t.Fatalf("unexpected load content: %q", string(loadResp.Part.InlineData.Data))
	}
	if loadResp.Part.InlineData.MIMEType != "text/plain" {
		t.Fatalf("unexpected mime type: %s", loadResp.Part.InlineData.MIMEType)
	}

	versionsResp, err := svc.Versions(ctx, &artifact.VersionsRequest{SessionID: "default", FileName: "test.txt"})
	if err != nil {
		t.Fatalf("Versions failed: %v", err)
	}
	if len(versionsResp.Versions) != 1 || versionsResp.Versions[0] != 1 {
		t.Fatalf("unexpected versions response: %#v", versionsResp.Versions)
	}

	if err := svc.Delete(ctx, &artifact.DeleteRequest{SessionID: "default", FileName: "test.txt"}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	listRespAfterDelete, err := svc.List(ctx, &artifact.ListRequest{SessionID: "default"})
	if err != nil {
		t.Fatalf("List after delete failed: %v", err)
	}
	if len(listRespAfterDelete.FileNames) != 0 {
		t.Fatalf("expected no files after delete, got: %#v", listRespAfterDelete.FileNames)
	}

	// Verify the file is removed from disk.
	if _, err := os.Stat(filepath.Join(baseDir, "default", "test.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, got: %v", err)
	}
}
