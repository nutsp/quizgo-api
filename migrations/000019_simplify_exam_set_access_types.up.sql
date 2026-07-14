UPDATE exam_sets
SET
    access_type = 'free',
    price_amount = 0,
    sale_price_amount = NULL,
    original_price_amount = NULL,
    allow_single_purchase = FALSE
WHERE access_type = 'trial';

UPDATE exam_sets
SET
    access_type = 'premium',
    allow_single_purchase = TRUE
WHERE access_type = 'paid';

UPDATE exam_sets
SET
    access_type = 'premium',
    price_amount = 0,
    sale_price_amount = NULL,
    original_price_amount = NULL,
    allow_single_purchase = FALSE
WHERE access_type = 'private';

UPDATE exam_sets
SET
    price_amount = 0,
    sale_price_amount = NULL,
    original_price_amount = NULL
WHERE access_type = 'premium'
  AND allow_single_purchase = FALSE;

ALTER TABLE exam_sets
DROP CONSTRAINT IF EXISTS exam_sets_access_type_check;

ALTER TABLE exam_sets
ADD CONSTRAINT exam_sets_access_type_check
CHECK (access_type IN ('free', 'premium'));
