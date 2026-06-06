package transfer

import (
	"fmt"

	"swoop/core/protocol"
)

// ValidateOfferFiles checks sender-claimed file metadata before an incoming
// offer is shown or accepted.
func ValidateOfferFiles(files []protocol.FileMeta) error {
	if len(files) == 0 {
		return fmt.Errorf("нет файлов")
	}
	if len(files) > protocol.MaxTransferFiles {
		return fmt.Errorf("слишком много файлов (макс %d)", protocol.MaxTransferFiles)
	}
	var total int64
	for i, f := range files {
		if f.Size < 0 {
			return fmt.Errorf("файл %d: отрицательный размер", i+1)
		}
		if f.Size > protocol.MaxTransferFileBytes {
			return fmt.Errorf("файл %q слишком большой", f.Name)
		}
		total += f.Size
		if total > protocol.MaxTransferTotalBytes {
			return fmt.Errorf("общий размер передачи превышает лимит")
		}
	}
	return nil
}
