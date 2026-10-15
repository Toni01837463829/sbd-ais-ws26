SELECT * FROM ecommerce ORDER BY price_per_unit DESC LIMIT 1;

SELECT COUNT(*)
FROM people_200k p1
JOIN people_200k p2
  ON p1.country = p2.country WHERE p1.id = p2.id;