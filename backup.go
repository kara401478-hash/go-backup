package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Result はバックアップ結果
type Result struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	FilesCopied int      `json:"files_copied"`
	FilesSkipped int     `json:"files_skipped"`
	TotalBytes int64     `json:"total_bytes"`
	Error      string    `json:"error,omitempty"`
	Success    bool      `json:"success"`
}

// Run はsrcからdstへバックアップを実行する
func Run(src, dst string) Result {
	result := Result{StartedAt: time.Now()}

	// コピー先にタイムスタンプ付きフォルダを作成
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dstPath := filepath.Join(dst, "backup_"+timestamp)

	if err := os.MkdirAll(dstPath, 0755); err != nil {
		result.Error = fmt.Sprintf("バックアップ先フォルダの作成に失敗: %v", err)
		result.FinishedAt = time.Now()
		return result
	}

	// srcを再帰的にコピー
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// コピー先のパスを計算
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// ファイルをコピー
		bytes, err := copyFile(path, targetPath)
		if err != nil {
			result.FilesSkipped++
			return nil // エラーでも続行
		}

		result.FilesCopied++
		result.TotalBytes += bytes
		return nil
	})

	if err != nil {
		result.Error = fmt.Sprintf("バックアップ中にエラー: %v", err)
	} else {
		result.Success = true
	}

	result.FinishedAt = time.Now()
	return result
}

// copyFile は1ファイルをコピーしてバイト数を返す
func copyFile(src, dst string) (int64, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

// FormatBytes はバイト数を人間が読みやすい形式に変換する
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
