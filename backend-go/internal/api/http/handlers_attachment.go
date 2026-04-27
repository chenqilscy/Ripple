package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AttachmentHandlers M4-B 节点附件（本地文件系统存储）。
//
// 端点：
//
//	POST /api/v1/attachments       multipart 上传，字段 file + 可选 node_id
//	GET  /api/v1/attachments/{id}  鉴权后回传二进制
//
// 安全：
//   - MaxBytesReader 限制请求体（UploadMaxMB）
//   - MIME 白名单
//   - 仅允许属主下载
type AttachmentHandlers struct {
	Repo        store.AttachmentRepository
	UploadDir   string
	MaxBytes    int64
	AllowedMIME map[string]bool
	Nodes       *service.NodeService
	Lakes       *service.LakeService
	Orgs        *service.OrgService
}

type attachmentOrgCounter interface {
	CountByOrg(ctx context.Context, orgID string) (int64, error)
}

type attachmentOrgSizer interface {
	SumSizeByOrg(ctx context.Context, orgID string) (int64, error)
}

// NewAttachmentHandlers 装配。
func NewAttachmentHandlers(repo store.AttachmentRepository, uploadDir string, maxMB int, allowMIME string) (*AttachmentHandlers, error) {
	if uploadDir == "" {
		return nil, errors.New("upload dir required")
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir upload dir: %w", err)
	}
	allow := map[string]bool{}
	for _, m := range strings.Split(allowMIME, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			allow[m] = true
		}
	}
	return &AttachmentHandlers{
		Repo:        repo,
		UploadDir:   uploadDir,
		MaxBytes:    int64(maxMB) * 1024 * 1024,
		AllowedMIME: allow,
	}, nil
}

// Upload POST /api/v1/attachments
func (h *AttachmentHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxBytes+4096) // 加 4KB 给 multipart 头

	if err := r.ParseMultipartForm(h.MaxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart or too large")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	mime := hdr.Header.Get("Content-Type")
	if !h.AllowedMIME[mime] {
		writeError(w, http.StatusUnsupportedMediaType, "mime not allowed: "+mime)
		return
	}
	if hdr.Size > h.MaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	nodeID := r.FormValue("node_id")
	orgID, err := h.resolveAttachmentOrg(r.Context(), u, nodeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	// M5-T3 安全加固：嗅探前 512 字节 magic bytes，拒绝伪造的 Content-Type
	sniffBuf := make([]byte, 512)
	nSniff, _ := io.ReadFull(file, sniffBuf)
	sniffed := http.DetectContentType(sniffBuf[:nSniff])
	if !mimeMatches(mime, sniffed) {
		writeError(w, http.StatusUnsupportedMediaType,
			"content-type mismatch: header="+mime+" sniffed="+sniffed)
		return
	}

	// 流式 sha256 + 写入临时文件
	tmpName := uuid.NewString()
	userDir := filepath.Join(h.UploadDir, u.ID)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir failed")
		return
	}
	ext := safeExt(hdr.Filename, mime)
	relPath := filepath.Join(u.ID, tmpName+ext)
	absPath := filepath.Join(h.UploadDir, relPath)

	dst, err := os.Create(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create file failed")
		return
	}
	hasher := sha256.New()
	// 把 sniff 缓冲先写出（不能 Seek，因为 multipart.File 不一定支持）
	if _, err := dst.Write(sniffBuf[:nSniff]); err != nil {
		_ = dst.Close()
		_ = os.Remove(absPath)
		writeError(w, http.StatusInternalServerError, "write head failed")
		return
	}
	hasher.Write(sniffBuf[:nSniff])
	written, err := io.Copy(io.MultiWriter(dst, hasher), file)
	_ = dst.Close()
	if err != nil {
		_ = os.Remove(absPath)
		writeError(w, http.StatusInternalServerError, "write failed")
		return
	}
	written += int64(nSniff)
	sha := hex.EncodeToString(hasher.Sum(nil))

	// 去重：同 user 同 sha 已存在 → 删新文件，返回旧记录
	if existing, err := h.Repo.GetBySHA(r.Context(), u.ID, sha); err == nil {
		_ = os.Remove(absPath)
		if existing.OrgID == orgID && existing.NodeID == nodeID {
			writeJSON(w, http.StatusOK, attachmentToDTO(existing))
			return
		}
		writeError(w, http.StatusConflict, "file already uploaded in a different attachment scope")
		return
	}
	if err := h.checkAttachmentQuota(r.Context(), orgID, written); err != nil {
		_ = os.Remove(absPath)
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	att := &store.Attachment{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		OrgID:     orgID,
		NodeID:    nodeID,
		MIME:      mime,
		SizeBytes: written,
		FilePath:  relPath,
		SHA256:    sha,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.Repo.Insert(r.Context(), att); err != nil {
		_ = os.Remove(absPath)
		writeError(w, http.StatusInternalServerError, "db insert failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, attachmentToDTO(att))
}

func (h *AttachmentHandlers) resolveAttachmentOrg(ctx context.Context, actor *domain.User, nodeID string) (string, error) {
	if nodeID == "" || h.Nodes == nil || h.Lakes == nil {
		return "", nil
	}
	node, err := h.Nodes.Get(ctx, actor, nodeID)
	if err != nil {
		return "", err
	}
	lake, _, err := h.Lakes.Get(ctx, actor, node.LakeID)
	if err != nil {
		return "", err
	}
	return lake.OrgID, nil
}

func (h *AttachmentHandlers) checkAttachmentQuota(ctx context.Context, orgID string, sizeBytes int64) error {
	if orgID == "" || h.Orgs == nil {
		return nil
	}
	counter, ok := h.Repo.(attachmentOrgCounter)
	if !ok {
		return fmt.Errorf("%w: attachment quota counter not configured", domain.ErrInvalidInput)
	}
	usedCount, err := counter.CountByOrg(ctx, orgID)
	if err != nil {
		return err
	}
	if err := h.Orgs.CheckQuota(ctx, orgID, domain.OrgQuotaAttachments, usedCount, 1); err != nil {
		return err
	}
	sizer, ok := h.Repo.(attachmentOrgSizer)
	if !ok {
		return fmt.Errorf("%w: attachment storage quota counter not configured", domain.ErrInvalidInput)
	}
	usedBytes, err := sizer.SumSizeByOrg(ctx, orgID)
	if err != nil {
		return err
	}
	usedMB := bytesToQuotaMB(usedBytes)
	totalMB := bytesToQuotaMB(usedBytes + sizeBytes)
	return h.Orgs.CheckQuota(ctx, orgID, domain.OrgQuotaStorageMB, usedMB, totalMB-usedMB)
}

func bytesToQuotaMB(n int64) int64 {
	if n <= 0 {
		return 0
	}
	const mb = int64(1024 * 1024)
	return (n + mb - 1) / mb
}

// Download GET /api/v1/attachments/{id}
func (h *AttachmentHandlers) Download(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}
	a, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 仅属主可下载（M4-S2 后续可扩展为湖成员可下载）
	if a.UserID != u.ID {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	// 防 path traversal
	cleanRel := filepath.Clean(a.FilePath)
	if strings.Contains(cleanRel, "..") {
		writeError(w, http.StatusInternalServerError, "invalid path")
		return
	}
	abs := filepath.Join(h.UploadDir, cleanRel)
	w.Header().Set("Content-Type", a.MIME)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, abs)
}

// mimeMatches 判断 header 声明的 MIME 与 sniffed MIME 是否兼容。
// 仅比较主类型/子类型，忽略 charset；JPEG 别名兼容。
func mimeMatches(declared, sniffed string) bool {
	d := strings.SplitN(strings.ToLower(declared), ";", 2)[0]
	s := strings.SplitN(strings.ToLower(sniffed), ";", 2)[0]
	if d == s {
		return true
	}
	jpegAlias := map[string]bool{"image/jpeg": true, "image/jpg": true, "image/pjpeg": true}
	return jpegAlias[d] && jpegAlias[s]
}

func safeExt(name, mime string) string {
	if i := strings.LastIndex(name, "."); i > 0 && len(name)-i <= 6 {
		ext := strings.ToLower(name[i:])
		// 仅允许字母数字
		ok := true
		for _, c := range ext[1:] {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				ok = false
				break
			}
		}
		if ok {
			return ext
		}
	}
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}

func attachmentToDTO(a *store.Attachment) map[string]any {
	return map[string]any{
		"id":         a.ID,
		"org_id":     a.OrgID,
		"node_id":    a.NodeID,
		"mime":       a.MIME,
		"size_bytes": a.SizeBytes,
		"sha256":     a.SHA256,
		"url":        "/api/v1/attachments/" + a.ID,
		"created_at": a.CreatedAt,
	}
}
