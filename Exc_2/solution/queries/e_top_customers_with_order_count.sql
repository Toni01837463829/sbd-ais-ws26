SELECT customer_name,
       COUNT(*) AS number_of_orders,
       SUM(price_per_unit * quantity) AS total_spent
FROM ecommerce
GROUP BY customer_name
ORDER BY total_spent DESC
LIMIT 5;
