package fiscalcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var ErrRecorded = errors.New("fiscal operation already reserved; read and reconcile original request")

// FileRecorder is an explicitly provisioned PRIVATE installation-level journal
// for a technical client. It is not the tenant HTTP API, the SQLite fiscal queue
// or an automatic part of SQLite backup. Integration must snapshot both stores
// consistently before enabling production; the current server remains disabled.
// It never stores private certificate keys or CSCs.
type FileRecorder struct{ directory string }

func NewFileRecorder(directory string) (*FileRecorder, error) {
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return nil, ErrInput
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, ErrInput
	}
	return &FileRecorder{directory: absolute}, nil
}
func journalName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
func (r *FileRecorder) write(name string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	path := filepath.Join(r.directory, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return ErrRecorded
	}
	if err != nil {
		return err
	}
	// A short/failed write is deliberately retained: an uncertain operation must
	// not become a new POST simply because its recovery record is unreadable.
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	directory, err := os.Open(r.directory)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func (r *FileRecorder) Reserve(ctx context.Context, in RecordedRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sum := sha256.Sum256(in.Body)
	if in.OperationID == "" || len(in.OperationID) > 128 || in.Scope == "" || in.RequestSHA256 != hex.EncodeToString(sum[:]) || len(in.Body) > 2*MaxXML {
		return ErrInput
	}
	return r.write(journalName(in.OperationID)+".request.json", in)
}
func (r *FileRecorder) Complete(ctx context.Context, in ExchangeResult) error {
	if in.OperationID == "" || (in.State != "uncertain" && in.State != "response_received") {
		return ErrInput
	}
	request, _, err := r.Read(in.OperationID)
	if err != nil || request.OperationID != in.OperationID {
		return ErrUncertain
	}
	if in.State == "response_received" {
		sum := sha256.Sum256(in.Body)
		if hex.EncodeToString(sum[:]) != in.SHA256 {
			return ErrInput
		}
	}
	return r.write(journalName(in.OperationID)+".response.json", in)
}
func privateJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4*MaxXML {
		return ErrInput
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return ErrInput
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4*MaxXML+1))
	if err != nil {
		return ErrInput
	}
	return DecodeInput(raw, target)
}
func (r *FileRecorder) Read(id string) (RecordedRequest, ExchangeResult, error) {
	var req RecordedRequest
	result := ExchangeResult{OperationID: id, State: "uncertain"}
	if err := privateJSON(filepath.Join(r.directory, journalName(id)+".request.json"), &req); err != nil {
		return req, result, err
	}
	sum := sha256.Sum256(req.Body)
	if req.OperationID != id || req.RequestSHA256 != hex.EncodeToString(sum[:]) {
		return RecordedRequest{}, result, ErrInput
	}
	response := filepath.Join(r.directory, journalName(id)+".response.json")
	if _, err := os.Lstat(response); errors.Is(err, os.ErrNotExist) {
		return req, result, nil
	}
	// Do not let an incomplete JSON inherit the fallback recovery identity.
	result = ExchangeResult{}
	if err := privateJSON(response, &result); err != nil {
		return req, ExchangeResult{OperationID: id, State: "uncertain"}, err
	}
	digest := sha256.Sum256(result.Body)
	if result.OperationID != id || (result.State != "response_received" && result.State != "uncertain") || (result.State == "response_received" && result.SHA256 != hex.EncodeToString(digest[:])) {
		return req, ExchangeResult{OperationID: id, State: "uncertain"}, ErrInput
	}
	return req, result, nil
}
