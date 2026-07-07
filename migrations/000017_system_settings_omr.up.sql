CREATE TABLE IF NOT EXISTS public.system_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL,
  description TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO public.system_settings (key, value, description)
VALUES (
  'omr_answer_sheet',
  '{
    "columns_per_row": 4,
    "questions_per_column": 5,
    "choice_labels": ["ก", "ข", "ค", "ง"],
    "show_header": true,
    "show_instructions": true,
    "show_examiner_info": true,
    "hold_to_answer_ms": 350,
    "sound_enabled_default": true
  }'::jsonb,
  'Global OMR answer sheet settings'
)
ON CONFLICT (key) DO NOTHING;
