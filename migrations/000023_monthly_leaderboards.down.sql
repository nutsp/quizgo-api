DROP TABLE IF EXISTS leaderboard_projection_failures;
DROP TABLE IF EXISTS leaderboard_awards;
DROP TABLE IF EXISTS leaderboard_entries;
DROP TABLE IF EXISTS leaderboard_scores;
DROP TABLE IF EXISTS leaderboard_season_exam_sets;
DROP TABLE IF EXISTS leaderboard_seasons;
ALTER TABLE exam_sets DROP COLUMN IF EXISTS published_at;
