-- Top 5 actors by total box-office gross, ranked with a window function.
WITH ranked AS (
  SELECT actor,
         total_gross,
         number_of_movies,
         RANK() OVER (ORDER BY total_gross DESC) AS rank
  FROM actor
)
SELECT rank, actor, total_gross, number_of_movies
FROM ranked
WHERE rank <= 5
ORDER BY rank;

-- Bucket actors by how many movies they made, and compare average gross.
SELECT CASE WHEN number_of_movies >= 50 THEN '50+ movies'
            WHEN number_of_movies >= 35 THEN '35-49 movies'
            ELSE 'under 35' END AS bucket,
       COUNT(*)                 AS actors,
       ROUND(AVG(total_gross), 1) AS avg_gross
FROM actor
GROUP BY bucket
ORDER BY avg_gross DESC;
