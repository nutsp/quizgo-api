package parser

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
	"virtual-exam-api/internal/questionimport/domain"
)

func parseXLSX(data []byte) (*ParseResult, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถอ่านไฟล์ Excel ได้")
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("ไฟล์ Excel ไม่มีชีต")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil || len(rows) == 0 {
		return nil, fmt.Errorf("ไฟล์ Excel ว่างเปล่า")
	}

	headers := rows[0]
	if err := validateHeaders(headers); err != nil {
		return nil, err
	}

	colIndex := buildColumnIndex(headers)
	var result []domain.ImportQuestionRow
	rowNum := 2

	for i := 1; i < len(rows); i++ {
		record := rows[i]
		if isEmptyRecord(record) {
			rowNum++
			continue
		}
		result = append(result, rowFromMap(rowNum, colIndex, record))
		rowNum++
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("ไม่พบข้อมูลคำถามในไฟล์")
	}

	return &ParseResult{Rows: result}, nil
}
