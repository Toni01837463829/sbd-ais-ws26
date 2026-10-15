SELECT product_category, SUM(price_per_unit * quantity) AS total_revenue FROM ecommerce
GROUP BY product_category
ORDER BY total_revenue DESC;