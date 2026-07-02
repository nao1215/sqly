-- JSONL rows land in a single "data" column; pull fields out with json_extract.
SELECT json_extract(data, '$.name') AS name,
       json_extract(data, '$.age')  AS age,
       json_extract(data, '$.city') AS city
FROM sample
WHERE json_extract(data, '$.age') >= 30
ORDER BY age DESC;
