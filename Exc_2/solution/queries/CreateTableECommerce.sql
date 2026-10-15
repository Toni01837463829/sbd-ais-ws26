DROP TABLE IF EXISTS ecommerce;

CREATE TABLE ecommerce (
  id SERIAL PRIMARY KEY,
  customer_name TEXT,
  product_category TEXT,
  quantity INTEGER,
  price_per_unit DECIMAL,
  order_date TIMESTAMP,
  country TEXT
);

COPY ecommerce(customer_name,product_category,quantity,price_per_unit,order_date,country) FROM '/data/orders_1M.csv' DELIMITER ',' CSV HEADER;