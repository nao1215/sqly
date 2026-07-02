-- Join a piped --stdin dataset (table "stdin") with a file argument.
-- Run with: cat user.csv | sqly --stdin csv --sql-file doc/vhs/join.sql identifier.csv
SELECT s.user_name, i.position
FROM stdin s
JOIN identifier i ON s.identifier = i.id
ORDER BY s.identifier;
