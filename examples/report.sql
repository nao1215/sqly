SELECT region, SUM(amount) AS total
FROM sales
GROUP BY region
ORDER BY total DESC;
