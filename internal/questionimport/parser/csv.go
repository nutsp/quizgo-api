package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"virtual-exam-api/internal/questionimport/domain"
)

func parseCSV(data []byte) (*ParseResult, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("ไฟล์ว่างเปล่า")
		}
		return nil, fmt.Errorf("ไม่สามารถอ่านไฟล์ CSV ได้")
	}
	if err := validateHeaders(headers); err != nil {
		return nil, err
	}

	colIndex := buildColumnIndex(headers)
	var rows []domain.ImportQuestionRow
	rowNum := 2

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ไม่สามารถอ่านแถวที่ %d ได้", rowNum)
		}
		if isEmptyRecord(record) {
			rowNum++
			continue
		}
		rows = append(rows, rowFromMap(rowNum, colIndex, record))
		rowNum++
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("ไม่พบข้อมูลคำถามในไฟล์")
	}

	return &ParseResult{Rows: rows}, nil
}

func isEmptyRecord(record []string) bool {
	for _, v := range record {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
