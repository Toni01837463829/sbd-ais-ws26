# Exercise 6: Card Sentinel


## 1. Decision (Part 1)


For this scenario we choose log-based CDC with Debezium.
The payment path belongs to another team and is certified, and CDC is the only option that leaves it completely unchanged: Debezium reads PostgreSQL's write-ahead log instead of the table, so the card processor never waits for a fraud check and the database gets no extra queries (R1).

Because every commit is in the WAL, Debezium also delivers every committed payment in commit order, so unlike polling it misses no payments that commit out of id order, and unlike dual write it never loses or invents a payment (R2).

The cost is the replication slot: PostgreSQL keeps all WAL that the connector has not read yet, so if the connector stops, for example over a weekend, the disk fills up and the database, and with it the payment path we wanted to protect, stops.

To notice this before it hurts, we would monitor the connector status (running or failed) and the amount of WAL the slot holds back, and alert as soon as that amount keeps growing, long before the disk is full.

If our team owned the payment path, we would choose the outbox pattern instead. Then changing the code is allowed, and the payment and its outbox event are written in the same transaction.
For example:
```sql
BEGIN;
INSERT INTO transactions (user_id, amount, ...) VALUES (4711, 32.74, ...);
INSERT INTO outbox (event_type, payload) VALUES ('PaymentCommitted', '{...}');
COMMIT;
```
so both are committed or neither is. On top of that, we would design the event format ourselves instead of exposing the table layout to every consumer.



## 2. Run 1: Debezium's defaults

**Prediction**

For Run 1 I'd expect that all three velocity instances share the work and R4 passes, because three instances at 400 to 500 payments/s each are enough for 1,000 payments/s.

**What I entered**

For Run 1 I entered:

```
docker compose up -d
```

Wait about 30 s, then open three terminals in `Exc_6\fraud`. Run `..\venv\Scripts\activate` in each one first.

Terminal 1:

```
python connector.py fraud-connector.json
```

```
removing what writes to fraud.* ...
registered fraud-connector-110142
connector fraud-connector-110142: RUNNING
  table public.transactions -> topic fraud.public.transactions   (appears with the first event)
```

```
python card_processor.py
```

```
run 1794909719: 1,000 transactions/s, a fraud every 8 s (first after 30 s). Ctrl+C to stop.
11:02:04      5,000 transactions     994/s
...
```

Terminal 2:

```
python agents.py
```

```
started 1 anomaly and 3 velocity agent(s). Ctrl+C stops all.
[vel-2] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[vel-3] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[ano-1] model anomaly, group anomaly-agent: fraud.public.transactions -> fraud-alerts
[vel-1] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[vel-2] assigned partitions [0]
[ano-1] assigned partitions [0]
[vel-2] assigned partitions [0]
[vel-2] assigned partitions []
[vel-3] assigned partitions []
[vel-1] assigned partitions [0]
[ano-1]     2,796 transactions      0 alerts    2,758 cardholders  lag   19.1 s  out-of-order 0
[vel-1]       491 transactions      0 alerts      489 cardholders  lag   17.2 s  out-of-order 0
[ano-1]     6,033 transactions      0 alerts    5,856 cardholders  lag   21.1 s  out-of-order 0
[vel-1]     1,833 transactions      0 alerts    1,814 cardholders  lag   20.7 s  out-of-order 0
...
```

Terminal 3, after about 75 s:

```
python scoreboard.py
```

```
Run 1794909719: started 11:01:59, 65,831 transactions in 75 s (877/s), 7 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   1        1    21,419
  velocity-agent           fraud.public.transactions                   1        3    49,331

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          26244 anomaly          46,286       37,086             0     36.8      12s ago
  vel-1          21992 velocity         17,567       16,129             2     60.6      12s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          0/2  (0%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  no alerts; worst lag of the velocity agents 60.6 s   FAIL  (<= 5 s)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          1/3  (33%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  27.7 s / 27.7 s   FAIL  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

**Interpretation**


1. We didn't tell Debezium how many partitions to make, so it made 1.
2. The velocity team has 3 members but there isonly 1 partition, so Kafka gave partition 0 to `vel-1`, and `vel-2` and `vel-3` got nothing:

```
[vel-1] assigned partitions [0]
[vel-2] assigned partitions []
[vel-3] assigned partitions []
```

3. `vel-1` alone does about 300 payments/s on my laptop, but 1,000/s arrive. Every second about 700 more payments pile up unread.
4. The pile (lag) grows:

```
[vel-1]  lag 17.2 s
[vel-1]  lag 34.2 s
[vel-1]  lag 51.3 s
```

`vel-1` is working on payments that are almost a minute old, and it falls further behind every second.

5. This is also what we see in the scoreboard:

```
velocity-agent   partitions 1   members 3   lag 49,331
```

That's 1 partition, 3 detectives, and 49,331 payments waiting.

**Result:**

* R4 FAIL (fresh within 5 s): the alert would come 60 s or later after the payment.
* R3 FAIL (catch 90 % of frauds): `vel-1` didn't miss the frauds. It simply hasn't reached those payments yet; they're still in the pile.

---

## 3. Run 2: more partitions

**Prediction**

For Run 2 I'd expect that all three velocity instances now get 2 partitions each, so they keep up and R4 passes. R3 should pass too, because now every payment is read by a velocity instance. I expect it to be close or maybe still not enough because before my laptop only managed about 300 payments/s.

**What I entered**

For Run 2 I added these two lines inside `"config"` of `fraud/fraud-connector.json`:

```json
    "topic.creation.default.partitions": "6",
    "topic.creation.default.replication.factor": "1"
```

Then I stopped the agents and the card processor with `Ctrl+C` and repeated the whole routine.

Terminal 1:

```
python connector.py fraud-connector.json
```

```
python card_processor.py
```

```
run 1794914733: 1,000 transactions/s, a fraud every 8 s (first after 30 s). Ctrl+C to stop.
12:25:38      4,800 transactions     942/s
...
12:27:53    140,288 transactions   1,001/s
```

Terminal 2:

```
python agents.py
```

```
started 1 anomaly and 3 velocity agent(s). Ctrl+C stops all.
[ano-1] model anomaly, group anomaly-agent: fraud.public.transactions -> fraud-alerts
[vel-1] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[vel-2] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[vel-3] model velocity, group velocity-agent: fraud.public.transactions -> fraud-alerts
[ano-1] assigned partitions [0, 1, 2, 3, 4, 5]
[vel-2] assigned partitions [2, 3]
[vel-1] assigned partitions [0, 1]
[vel-3] assigned partitions [4, 5]
[ano-1]     3,441 transactions      0 alerts    3,368 cardholders  lag    3.2 s  out-of-order 0
[vel-1]     1,285 transactions      0 alerts    1,273 cardholders  lag    1.2 s  out-of-order 0
[vel-2]     1,306 transactions      0 alerts    1,295 cardholders  lag    1.2 s  out-of-order 0
[vel-3]     1,315 transactions      0 alerts    1,301 cardholders  lag    1.3 s  out-of-order 0
[ano-1]     7,335 transactions      0 alerts    7,058 cardholders  lag    4.0 s  out-of-order 0
[vel-1]     2,722 transactions      0 alerts    2,678 cardholders  lag   11.2 s  out-of-order 14
[vel-2]     2,707 transactions      0 alerts    2,668 cardholders  lag   10.6 s  out-of-order 16
[vel-3]     2,704 transactions      0 alerts    2,665 cardholders  lag   11.2 s  out-of-order 9
...
[vel-1]    13,435 transactions      0 alerts   12,529 cardholders  lag    0.4 s  out-of-order 47
[vel-2]    13,413 transactions      0 alerts   12,570 cardholders  lag    0.4 s  out-of-order 29
[vel-3]    13,401 transactions      0 alerts   12,540 cardholders  lag    0.4 s  out-of-order 41
...
[ano-1]    48,265 transactions      1 alerts   38,171 cardholders  lag   12.2 s  out-of-order 0
[vel-1]    16,478 transactions      0 alerts   15,123 cardholders  lag    9.8 s  out-of-order 75
[vel-2]    16,405 transactions      0 alerts   15,148 cardholders  lag   10.0 s  out-of-order 50
[vel-3]    16,349 transactions      0 alerts   15,096 cardholders  lag    9.8 s  out-of-order 66
...
[vel-2]    49,409 transactions      0 alerts   38,894 cardholders  lag    2.9 s  out-of-order 141
[vel-3]    49,759 transactions      0 alerts   39,248 cardholders  lag    2.7 s  out-of-order 131
[ano-1]   144,818 transactions      7 alerts   76,162 cardholders  lag   15.9 s  out-of-order 0
[vel-1]    49,501 transactions      0 alerts   38,885 cardholders  lag    2.7 s  out-of-order 146
...
```

Terminal 3, after almost 3 minutes:

```
python scoreboard.py
```

```
Run 1794914733: started 12:25:33, 165,209 transactions in 165 s (1,001/s), 18 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1    11,158
  velocity-agent           fraud.public.transactions                   6        3     4,765

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          25304 anomaly         157,902       79,070             0     17.8       9s ago
  vel-1           2648 velocity         54,249       41,723           177     11.0       9s ago
  vel-2          24712 velocity         54,203       41,763           177     11.4       9s ago
  vel-3          18900 velocity         54,573       42,056           167     11.1       9s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          0/9  (0%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  no alerts; worst lag of the velocity agents 11.4 s   FAIL  (<= 5 s)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          8/9  (89%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  12.2 s / 13.4 s   FAIL  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

**Interpretation**

1. Debezium now made 6 partitions, so every velocity instance got 2 of them and all three work:

```
[vel-1] assigned partitions [0, 1]
[vel-2] assigned partitions [2, 3]
[vel-3] assigned partitions [4, 5]
```

2. Each velocity instance does about 350 payments/s on my laptop, so together about 1,050/s. That is enough for the 1,000/s that arrive: the pile does not grow like in Run 1, and the lag keeps going back down to about 0 to 3 s:

```
[vel-1]  lag    1.2 s
[vel-1]  lag   11.2 s
[vel-1]  lag    0.4 s
[vel-1]  lag    9.8 s
[vel-1]  lag    2.7 s
```

3. Each model can only catch its own kind of fraud, and the scoreboard shows them separately:

```
velocity model (card testing)
    R3  frauds caught          0/9  (0%)   FAIL  (>= 90 %)

anomaly model (account takeover)
    R3  frauds caught          8/9  (89%)   FAIL  (>= 90 %)
```

| Fraud | What the thief does | Who can catch it | Run 2 |
|---|---|---|---|
| card testing | 12 small payments in about 20 s | only velocity | 0/9 |
| account takeover | one huge payment | only anomaly | 8/9 |

4. Velocity counts: "how many payments did this cardholder make in the last 60 s?" It alerts at the 8th.
5. The thief makes 12 payments with stolen card 4711. Each payment has a different `id`, and the key is still the `id`, so Kafka distributes them by `id` over the 6 partitions:

```
payment id 1001 (card 4711) -> partition 3 -> vel-2
payment id 1847 (card 4711) -> partition 0 -> vel-1
payment id 2290 (card 4711) -> partition 5 -> vel-3
payment id 3012 (card 4711) -> partition 1 -> vel-1
... (12 payments in total)
```

In the end, each instance has seen only about 4 of the 12:

| Instance | Payments of card 4711 it saw |
|---|---|
| vel-1 | 4 | 
| vel-2 | 4 |
| vel-3 | 4 |

Each instance has its own memory and does not know what the others saw. Nobody counts to 8, so there is no alert, although all 12 payments were scored.

6. Anomaly also remembers per cardholder (the last 50 amounts). But it runs as one instance that reads all 6 partitions, so it sees every payment of every cardholder, and it still catches its frauds (8/9).

**Result:**

* R3 FAIL (catch 90 % of frauds): 0/9. This time the frauds are not waiting in the pile, the velocity instances read them, but each one sees only a part of each attack.
* R4 FAIL (fresh within 5 s): most of the time the lag is only 0 to 3 s, but the scoreboard counts the worst lag (11.4 s), and those short jumps to about 10 s are enough to fail.

---

## 4. Run 3: the cardholder as the key

**Prediction**

For Run 3 I'd expect that the key of an event is now `{"user_id": ...}` instead of `{"id": ...}`. All payments of one cardholder then land in the same partition, so one velocity instance sees all 12 test payments of a stolen card and R3 passes.

**What I entered**

For Run 3 I added this line inside `"config"` of `fraud/fraud-connector.json` (now it is the same as `solution/fraud-connector.json`):

```json
    "message.key.columns": "public.transactions:user_id"
```

Then I stopped the agents and the card processor with `Ctrl+C` and repeated the whole routine. My laptop could not keep up with 1,000 payments/s (see Run 2), so for this run I started the card processor with `--tps 500`, as the README suggests for slower laptops.

Terminal 1:

```
python connector.py fraud-connector.json
```

```
python card_processor.py --tps 500
```

Terminal 2:

```
python agents.py
```

Terminal 3, after about 2.5 minutes:

```
python scoreboard.py
```

```
Run 1794917578: started 13:12:58, 75,147 transactions in 150 s (501/s), 16 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     1,736
  velocity-agent           fraud.public.transactions                   6        3     1,674

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          26472 anomaly          73,946       52,133             0      0.1       2s ago
  vel-1          11980 velocity         23,924       16,972             2      0.1       2s ago
  vel-2          26736 velocity         23,843       17,060             5      0.1       2s ago
  vel-3          24988 velocity         23,981       17,020             2      0.1       2s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          7/7  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.1 s / 0.9 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          8/8  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.2 s / 1.0 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

**Interpretation**

1. The key of an event is now the cardholder, `{"user_id": ...}` (checked in Kafka), and there are still 6 partitions, 2 per velocity instance.
2. Kafka now distributes the payments by `user_id`. All 12 payments of stolen card 4711 have the same key, so they all land in the same partition and are read by the same instance:

```
payment id 1001 (card 4711) -> partition 3 -> vel-2
payment id 1847 (card 4711) -> partition 3 -> vel-2
payment id 2290 (card 4711) -> partition 3 -> vel-2
... (all 12 payments)
```

3. That instance counts all 12 payments of the card, reaches 8 and raises an alert. The velocity model caught all 7 card-testing frauds (Run 2: 0), with 0 false alerts.
4. At 500 payments/s, the three velocity instances (about 350 payments/s each on my laptop) have more than enough capacity, so nothing piles up. Every instance scores the payments almost immediately (lag 0.1 s), and an alert comes about 1 s after the payment (p95 0.9 s):

```
velocity-agent   partitions 6   members 3   lag 1,674
vel-1   lag 0.1 s     vel-2   lag 0.1 s     vel-3   lag 0.1 s
```

The group lag of 1,674 payments (about 3 s at 500/s) is not a real pile: the agents save their position in Kafka only every 5 s (see `fraud_agent.py`), so the group lag always shows up to 5 s of payments, even when the agents are up to date.

**Result:**

* R3 PASS (catch 90 % of frauds): 7/7 (100 %), 0 false alerts. All payments of a card meet in one instance.
* R4 PASS (fresh within 5 s): alert latency p95 0.9 s.
* R2 PASS: both models have their own consumer group and get every payment; the anomaly model also passes (8/8, p95 1.0 s).

This is the final design: 6 partitions so that the instances can share the work, and the cardholder as the key so that all payments of a cardholder meet in one instance.

---

## 5. Closing answer

With 1 partition, only one velocity instance would have worked, so the alerts would have come later and later during the day, and adding more instances would not have helped at all. With the payment `id` as the key, everything would have looked healthy (all instances busy, no lag), but the velocity model would have caught almost no card testing, and the stolen cards would not have been blocked. Neither problem gives an error message, nothing crashes, so the team would only have noticed by looking: in the consumer group, more members than partitions means idle instances, and a growing lag means the model is too slow. The key of one message in the Kafka UI shows `{"id": ...}` instead of the cardholder, and testing with a few known frauds before going live (like the scoreboard does) would have shown that the velocity model catches none of them.
