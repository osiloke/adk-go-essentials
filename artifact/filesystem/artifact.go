package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath" 
	"time"

	"github.com/osiloke/adk-go-essentials/observability"
	"google.golang.org/adk/artifact"
	"google.golang.org/genai"
)

// FileSystemArtifactService implements artifact.Service using the local file system.
type FileSystemArtifactService struct {
	baseDir string
}

// NewFileSystemArtifactService creates a new FileSystemArtifactService.
func NewFileSystemArtifactService(baseDir string) artifact.Service {
	_ = os.MkdirAll(baseDir, 0755)
	return &FileSystemArtifactService{baseDir: baseDir}
}

// BaseDir returns the base directory for artifact storage.
func (s *FileSystemArtifactService) BaseDir() string {
	return s.baseDir
}

func (s *FileSystemArtifactService) getSessionDir(sessionID string) string {
	if sessionID == "" {
		sessionID = "default"
	}
	return filepath.Join(s.baseDir, sessionID)
}

func (s *FileSystemArtifactService) getPath(sessionID, fileName string) string {
	return filepath.Join(s.getSessionDir(sessionID), fileName)
}

// Save saves an artifact to the file system within a session directory.
func (s *FileSystemArtifactService) Save(ctx context.Context, req *artifact.SaveRequest) (*artifact.SaveResponse, error) {
	if req.Part == nil || req.Part.InlineData == nil {
		return nil, fmt.Errorf("content or inline data is nil")
	}

	sessionDir := s.getSessionDir(req.SessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	filePath := filepath.Join(sessionDir, req.FileName)

	if err := os.WriteFile(filePath, req.Part.InlineData.Data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Versioning stub: use timestamp
	return &artifact.SaveResponse{Version: time.Now().Unix()}, nil
}

// List lists artifacts in the session directory.
func (s *FileSystemArtifactService) List(ctx context.Context, req *artifact.ListRequest) (*artifact.ListResponse, error) {
	sessionDir := s.getSessionDir(req.SessionID)
	observability.Log.Debug("[Artifact] List requested", "session_id", req.SessionID, "session_dir", sessionDir)

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			observability.Log.Debug("[Artifact] Session directory does not exist", "session_id", req.SessionID, "session_dir", sessionDir)
			return &artifact.ListResponse{FileNames: []string{}}, nil
		}
		observability.Log.Error("[Artifact] Failed to read session directory", "session_id", req.SessionID, "session_dir", sessionDir, "error", err)
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	observability.Log.Debug("[Artifact] Listed artifacts", "session_id", req.SessionID, "count", len(names), "files", names)
	return &artifact.ListResponse{FileNames: names}, nil
}

// Load loads an artifact from the session directory.
func (s *FileSystemArtifactService) Load(ctx context.Context, req *artifact.LoadRequest) (*artifact.LoadResponse, error) {
	filePath := s.getPath(req.SessionID, req.FileName)
	observability.Log.Debug("[Artifact] Load requested", "session_id", req.SessionID, "file_name", req.FileName, "file_path", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			observability.Log.Warn("[Artifact] File not found", "session_id", req.SessionID, "file_name", req.FileName, "file_path", filePath)
		} else {
			observability.Log.Error("[Artifact] Failed to read file", "session_id", req.SessionID, "file_name", req.FileName, "file_path", filePath, "error", err)
		}
		return nil, err
	}

	mimeType := "application/octet-stream"
	// Optional: basic mime type detection extension
	ext := filepath.Ext(req.FileName)
	switch ext {
	case ".md":
		mimeType = "text/markdown"
	case ".json":
		mimeType = "application/json"
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".txt":
		mimeType = "text/plain"
	}

	observability.Log.Debug("[Artifact] File loaded successfully", "session_id", req.SessionID, "file_name", req.FileName, "mime_type", mimeType, "size_bytes", len(data))
	return &artifact.LoadResponse{
		Part: &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: mimeType,
				Data:     data,
			},
		},
	}, nil
}

// Delete deletes an artifact from the session directory.
func (s *FileSystemArtifactService) Delete(ctx context.Context, req *artifact.DeleteRequest) error {
	filePath := s.getPath(req.SessionID, req.FileName)
	return os.Remove(filePath)
}

// Versions lists versions of an artifact (Stub).
func (s *FileSystemArtifactService) Versions(ctx context.Context, req *artifact.VersionsRequest) (*artifact.VersionsResponse, error) {
	filePath := s.getPath(req.SessionID, req.FileName)
	if _, err := os.Stat(filePath); err == nil {
		return &artifact.VersionsResponse{Versions: []int64{1}}, nil
	}
	return &artifact.VersionsResponse{Versions: []int64{}}, nil
}
