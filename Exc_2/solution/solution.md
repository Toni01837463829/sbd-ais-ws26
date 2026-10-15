# Exercise

## Table generation

```sql
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

\COPY ecommerce(customer_name,product_category,quantity,price_per_unit,order_date,country) FROM '/data/orders_1M.csv' DELIMITER ',' CSV HEADER;
```

## Does LIMIT 10 make queries cheaper?


Query tested for the comparison:

```sql
SELECT department, AVG(salary)
FROM people_big
GROUP BY department;

SELECT department, AVG(salary)
FROM people_big
GROUP BY department
LIMIT 10;
```

Result: execution times were in the same range for repeated runs.
- LIMIT only saves work when PostgreSQL can stop reading early, e.g. SELECT * FROM table LIMIT 10 without sorting or grouping.
- With GROUP BY or ORDER BY, the full table must be scanned and aggregated before LIMIT is applied, so the runtime is about the same.
## Activity 2
### A. Which order has the highest price_per_unit?


```sql
SELECT * FROM ecommerce ORDER BY price_per_unit DESC LIMIT 1;
```

### B. Top 3 product categories by total quantity sold

```sql
SELECT product_category, SUM(quantity) AS summed_up_quantity FROM ecommerce
GROUP BY product_category
ORDER BY summed_up_quantity DESC
LIMIT 3;
```

### C. Total revenue per product category (price_per_unit * quantity)


```sql
SELECT product_category, SUM(price_per_unit * quantity) AS total_revenue FROM ecommerce
GROUP BY product_category
ORDER BY total_revenue DESC;
```

### D. Top 5 customers by total spending


```sql
SELECT customer_name, SUM(price_per_unit * quantity) AS total_spent
FROM ecommerce
GROUP BY customer_name
ORDER BY total_spent DESC
LIMIT 5;
```

### E. Spending totals of the top 5 customers and their order counts


```sql
SELECT customer_name,
       COUNT(*) AS number_of_orders,
       SUM(price_per_unit * quantity) AS total_spent
FROM ecommerce
GROUP BY customer_name
ORDER BY total_spent DESC
LIMIT 5;
```
**What do i notice?**

- Customer names are random combinations of 33 first names and 34 last names, so there are only 33 × 34 = 1,122 distinct names.
- With 1,000,000 orders, each name appears in about 1,000,000 / 1,122 ≈ 890 orders.
- Category, quantity, price and country are chosen at random for each order => the totals average out. The top 5 are the names that happened to get slightly higher totals by chance.

## Activity 2.2

### Step 1

| rows in table | join result (`COUNT(*)`) | time |
|---|---|---|
| 50 000 | 27 501 822 | 2 458 ms |
| 100 000 | 109 946 508 | 10 252 ms |
| 200 000 | 439 395 606 | 52 841 ms |

**Growth when the input doubles:**

- Count: 109 946 508 / 27 501 822 ~ 4.0x, and 439 395 606 / 109 946 508 ~ 4.0x
- Time: 10 252 / 2 458 ~ 4.2x, and 52 841 / 10 252 ~ 5.2x

**Why it is so slow:**

The Join is a crossjoin => that means every row in table 1 we pair with every row in table 2.
for n rows we get a result of n*n rows.

Doubeling n therefore results in 4x the number of pairs after the Join.

Runtime also follows about the same trend - for bigger numbers it could get even slower because of some memory,... problems for super large queries.

**Prediction for 1M rows:**

- Input is 5x the 200 000 row table, so the result is 5*5 = 25x larger:

- Rows: 439 395 606 * 25 = 10 984 890 150 
- Time: 52.8s * 25 ~ 22 minutes+


### Step 2

Create the index:

```sql
CREATE INDEX idx_people_100k_country ON people_100k (country);
```

The index was used `->  Parallel Index Only Scan using idx_people_100k_country on people_100k p1  (cost=0.29..1596.52 rows=58824 width=8) (actual time=0.024..159.675 rows=50000.00 loops=2)` but the output was not faster. An index helps to find individual rows. In this example we have to count all rows either way.

### Step 3

Rewritten query:

```sql
SELECT SUM(cnt * cnt) AS pairs
FROM (
  SELECT COUNT(*) AS cnt
  FROM people_100k
  WHERE country IS NOT NULL
  GROUP BY country
) t;
```

We make a subselect where we get the count of each country then multiply each countries count by itself and sum over all countries.

eg Austria = 2, Germany = 3 => 2x2 + 3x3

We get the exact same number of outputs:
```
postgres=# SELECT SUM(cnt * cnt) AS pairs
FROM (
  SELECT COUNT(*) AS cnt
  FROM people_100k
  GROUP BY country         
) t;
   pairs   
-----------
 109946508
(1 row)

postgres=# SELECT COUNT(*)                                
FROM people_100k p1
JOIN people_100k p2 ON p1.country = p2.country;
   count   
-----------
 109946508
(1 row)
```

Result of this query on people_big:
```
-------------
 10983941260
(1 row)

Time: 1128.136 ms (00:01.128)
```

Select took ~1s on people_big instead of the 22minutes we expected with the other select

### Step 4


More hardware/better hardware only makes the individual steps run faster. EG doubling cpus makes the task run twice as fast but the Join still grows in n*n time while the Subselect grows in n.
Also the index does not help. The Program still has to look at each row once to count it. An index is only good for finding specific rows.

**If the business actually needs the pairs stored:** This takes an enormouse amount of storage. Making the machine it is stored on bigger only linearly increases the storage, while adding new records increases storage required squared. Also a search tree like a B-tree would not help us. finding pairs/storing pairs is not easier/faster when you use a B-tree, a B-tree only helps finding indivudual rows not finding all rows.

What you could do in this case to improve performance would be to cluster, eg. to partition the dataset based on country, so that each countries records are stored on a different node, retrieving would let multiple nodes work simultaniously in that case.

Limits of OLTP would be that it has to load milions of rows. Even if you split the data across many machines. OLTP has to load a huge number of rows which will get slow.
