-- One query over two files in two formats: rows from a CSV, names from a JSONL.
-- A JSONL line arrives as one `data` column holding the document, so its fields
-- are read with json_extract.
SELECT
    json_extract(r.data, '$.name') AS region_name,
    json_extract(r.data, '$.lead') AS lead,
    SUM(s.amount)                  AS total
FROM sales AS s
JOIN regions AS r
    ON s.region = json_extract(r.data, '$.region')
GROUP BY region_name, lead
ORDER BY total DESC;
