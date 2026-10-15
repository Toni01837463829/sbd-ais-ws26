SELECT product_category, SUM(quantity) AS summed_up_quantity FROM ecommerce
GROUP BY product_category
ORDER BY summed_up_quantity DESC
LIMIT 3;