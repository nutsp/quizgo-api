package usecase

import (
	"fmt"

	"virtual-exam-api/internal/media/storage"
	"virtual-exam-api/internal/questionimport/domain"
	"virtual-exam-api/internal/questionimport/zipimages"
)

func resolveImportImageURLs(
	store *storage.LocalStorage,
	jobID string,
	row domain.ImportQuestionRow,
	images map[string][]byte,
) (domain.ImportQuestionRow, error) {
	if store == nil {
		return row, nil
	}
	subdir := fmt.Sprintf("questions/import-%s", jobID)

	var err error
	if row.QuestionImage != "" {
		row.QuestionImageURL, err = uploadNamedImage(store, subdir, row.QuestionImage, images)
		if err != nil {
			return row, err
		}
	}
	if row.ExplanationImage != "" {
		row.ExplanationImageURL, err = uploadNamedImage(store, subdir, row.ExplanationImage, images)
		if err != nil {
			return row, err
		}
	}
	if row.ChoiceAImage != "" {
		row.ChoiceAImageURL, err = uploadNamedImage(store, subdir, row.ChoiceAImage, images)
		if err != nil {
			return row, err
		}
	}
	if row.ChoiceBImage != "" {
		row.ChoiceBImageURL, err = uploadNamedImage(store, subdir, row.ChoiceBImage, images)
		if err != nil {
			return row, err
		}
	}
	if row.ChoiceCImage != "" {
		row.ChoiceCImageURL, err = uploadNamedImage(store, subdir, row.ChoiceCImage, images)
		if err != nil {
			return row, err
		}
	}
	if row.ChoiceDImage != "" {
		row.ChoiceDImageURL, err = uploadNamedImage(store, subdir, row.ChoiceDImage, images)
		if err != nil {
			return row, err
		}
	}
	return row, nil
}

func uploadNamedImage(store *storage.LocalStorage, subdir, filename string, images map[string][]byte) (string, error) {
	data, ok := zipimages.LookupImage(images, filename)
	if !ok {
		return "", fmt.Errorf("ไม่พบไฟล์รูปภาพ: %s", filename)
	}
	return store.SaveImage(subdir, filename, data)
}
