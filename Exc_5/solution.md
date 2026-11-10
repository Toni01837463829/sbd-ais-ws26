# Exercise 5: Solution

Fill out everything marked with `TODO`. See `stepbystep.md` for what to do.

---

## Stage A: Today's system

### A1: Small plant (20 machines, 200 readings per second)

**Commands**

```
python baseline_detector.py
python plant.py --sink postgres --machines 20
docker exec -it postgres psql -U postgres -d factory -c "SELECT count(*) FROM readings WHERE run = (SELECT max(run) FROM readings);"
```

**Output: last line of the plant**

```
run 1794298381: sent 37,300 readings in 187 s
```

**Output: last line of the detector**

```
[baseline] stopped: read 32,434 readings, last id 37,300, 4,866 readings missing,931 anomalies
```

**Output: count from the table**

```
 count 
-------
 37300
(1 row)
```

**Results**

| Value | Number |
|---|---|
| Readings sent by the plant | 37300 |
| Rows in the table (`count`) | 37300 |
| Readings the detector read (`read`) | 32434 |
| Readings missing (`missing`) | 4866 |
| Backlog at the end | 0 |

**Are the missing readings in the table?**

Yes, the Select of the Count shows all readings are there

**Why did the detector never see them?**

* A row gets its `id` when it is inserted, but others can only see it after the commit.
The four gateways work at the same time, and each one waits up to 120 ms before its commit. So rows do not become visible in `id` order.

| Time | Gateway 1 | Gateway 2 | Detector |
|---|---|---|---|
| 1 | inserts 100-114, no commit yet | | |
| 2 | | inserts 115-129, commits | |
| 3 | | | polls: sees only 115-129, remembers `last_id = 129` |
| 4 | commits 100-114 | | |
| 5 | | | polls `id > 129`: never gets 100-114 |

So the rows 100-114 are in the table, but the detector has already moved past them.

Timeline: gateway 1 inserts 100-114 and commits 100 ms later. Gateway 2 inserts 115-129 and commits at once.

- **Which rows does the detector see when it polls in between?**
  Only rows 115-129, because only gateway 2 has committed. Rows 100-114 already exist, but nobody else can see them before the commit.
- **Which `id` does it remember?**
  `last_id = 129`, the highest id it has read.
- **What happens to rows 100-114?**
  Gateway 1 commits them later, so they are in the table. But the detector next asks for `id > 129`, so it never reads them. They are lost for the detector and counted as `missing`.

**Which requirement does this break, and why?**

R2 (complete and ordered). The model does not get every reading of a machine.
The reason is not speed, because the detector kept up (backlog 0).
The reason is that ids are given out when a row is inserted, but rows become visible when they are committed, and that is a different order.

### A2: Full plant (60 machines, 600 readings per second)

**Commands**

```powershell
python baseline_detector.py
python plant.py --sink postgres
```

**Output: detector lines while the plant runs**

```text
[baseline] polling table readings for id > 37300 (Ctrl+C to stop)
[baseline] read        0  backlog       0 rows  delay   0.0 s  missing      0  anomalies 0
[baseline] read      659  backlog     900 rows  delay   1.7 s  missing      1  anomalies 0
[baseline] read    3,329  backlog   4,050 rows  delay   6.9 s  missing     31  anomalies 0
[baseline] read    5,329  backlog   6,400 rows  delay  10.8 s  missing     62  anomalies 0
[baseline] read    7,329  backlog   8,721 rows  delay  14.6 s  missing     62  anomalies 0
[baseline] read    9,329  backlog  10,859 rows  delay  18.3 s  missing     62  anomalies 0
[baseline] read   11,329  backlog  13,149 rows  delay  22.2 s  missing     62  anomalies 0
[baseline] read   13,329  backlog  15,485 rows  delay  25.9 s  missing     62  anomalies 0
[baseline] read   15,329  backlog  17,729 rows  delay  29.7 s  missing     62  anomalies 0
[baseline] read   17,329  backlog  19,944 rows  delay  33.4 s  missing     62  anomalies 0
[baseline] read   19,329  backlog  22,235 rows  delay  37.1 s  missing     62  anomalies 0
[baseline] read   21,329  backlog  24,509 rows  delay  40.9 s  missing     62  anomalies 0
[baseline] read   23,329  backlog  27,009 rows  delay  45.2 s  missing     62  anomalies 0
[baseline] read   25,329  backlog  29,451 rows  delay  49.2 s  missing     62  anomalies 0
[baseline] read   27,329  backlog  31,769 rows  delay  53.1 s  missing     62  anomalies 0
[baseline] ANOMALY M-029 seq=479 score=5.0
... (more ANOMALY lines for M-029)
[baseline] read   29,329  backlog  34,029 rows  delay  56.8 s  missing     62  anomalies 10
[baseline] read   31,329  backlog  36,470 rows  delay  60.9 s  missing     62  anomalies 45
[baseline] read   33,329  backlog  38,849 rows  delay  64.9 s  missing     62  anomalies 78
[baseline] stopped: read 33,329 readings, last id 71,020, 62 readings missing, 83 anomalies
```

Plant: `run 1794299357: sent 74,580 readings in 125 s` (600 readings/s, faults on M-029 at 40 s and M-005 at 100 s).

**Results**

| Time | `read` | `backlog` | `delay` | `missing` |
|---|---|---|---|---|
| about 10 s | 5,329 | 6,400 | 10.8 s | 62 |
| about 30 s | 15,329 | 17,729 | 29.7 s | 62 |
| about 60 s | 31,329 | 36,470 | 60.9 s | 62 |

**What is different from A1?**

Only the plant size: 60 machines instead of 20, so 600 readings per second instead of 200.
At 200 the detector keeps up; at 600 it does not.

**What happens to `delay`, and why?**

It grows all the time, from 1.7 s to 64.9 s. The detector reads older and older rows, because it is further behind every second.

**What happens to `backlog`, and why?**

It grows all the time, from 900 to 38,849 rows. 600 rows arrive per second, but only about 300 can be scored, so the pile keeps growing.

**What happens to `missing`, and why?**

It grows at first (1 => 31 => 62), then stays at 62.
Once the detector is behind, the rows it reads were committed long ago, so there are no more gaps.
This is not an improvement: the detector only stopped losing rows because it is too late.

**Calculation: how many readings per second can one detector score?**

The model takes 3 ms per reading: 1 / 0.003 s = in theory about 333 readings per second
The plant sends 600 per second, so one detector can do only about half of the work.

**Which requirements does this break?**

- R1 (scale): one detector cannot handle even 600 readings per second, let alone 100,000.
- R3 (fresh): the fault on M-029 started at t = 40 s, but the first anomaly came about 60 s late. R3 asks for at most 5 s.
- R2 is still broken too (62 readings missing), as shown in A1.

**In short:** A1 shows the system loses data (R2). A2 shows it is too slow (R1, R3).

---

## Stage B: Decision record

### The 2 options
**Option 1:** keep PostgreSQL, but poll better

For example, use several detectors, and each one polls only some of the machines.

- R1: risky. 100,000 rows per second into one table is huge, and every poller adds more queries => big load writing and storing the data.
- R2: the problem from A1 stays: rows become visible in commit order, not id order. Fixing this needs extra work to get around that.
- R3: works for a small plant, gets risky for a big one (many transactions(reads,writes) will delay how fast you get your answer).
- R4: this fails. Every new reader (archive, dashboard) sends more queries to PostgreSQL. - should not be the case.

**Option 2:** the machines write into Kafka

- R1: OK. You can add more partitions and more detectors.
- R2: OK. Key = machine_id, so one machine always lands in the same partition, in order. A message gets its offset when it is written, so there are no invisible rows.
- R3: OK, you just have to make sure there are enough detectors reading the data fast enough.
- R4: OK. Archive and dashboard are just their own consumer groups, like billing in Part 1. Nobody notices them.

### Decision

I would choose **Option 2: Kafka**.

- **Machines:** they only measure and send their readings to the topic `readings`, with key `machine_id`.
  60 machines * 10 readings/s = 600 readings/s in total. The machines do not know about anomalies.
- **Topic `readings`:** 6 partitions. The key decides the partition, so all readings of one machine land in the same partition, in order.
- **Detectors:** one consumer group ("line-guardian") with 3 detectors. The group splits the partitions,
  so each detector handles about a third of the machines (about 200 readings/s, below its limit of about 300/s).
  Each machine is checked by exactly one detector, which matters because the model remembers each machine's history.
- **Topic `anomalies`:** a detector writes here only when the detectors model detects a fault, not for normal readings.
  The maintenance team reads the alerts from here.

PostgreSQL stays the system of record for the machine registry (the `machines` table). It just no longer stores and serves every sensor reading.

### Numbers

| Rate | Calculation | Detector instances I would use |
|---|---|---|
| 600 readings/s | 1 detector: 1 / 0.003 s = about 333 readings/s. 600 / 333 = 1.8 - we need at least 2 | **3** (with 6 partitions) |
| 100,000 readings/s | 100,000 / 333 = 300 => at least 300 | around 400 (with about 500 partitions, on different kafka servers) |

### Why I use this many instances and not just the minimum:

- **Real cost is higher:** a best case reading takes 3ms, in reality we can expect some to take 4ms, ... so each detector does a bit less, not 333.
- **Machines are not spread perfectly even:** the key decides the partition, so some partitions could get more machines than others.
  With 6 partitions and only 2 detectors, one detector would get 36 machines = 360 readings/s, which is too much. With 3 detectors the most is 25 machines = 250 readings/s.
- **Catching up:** after a restart or a rebalance, a detector must read faster than new data arrives, otherwise it never catches up.
- **Partitions >= detectors:** one partition can only have one reader in a group. Extra detectors would just be idle, so the topic needs at least as many partitions as detectors.

### Tipping point

**When is option 1 (PostgreSQL) enough, and when do you need option 2?**

This is a rough estimate.

**PostgreSQL is enough** when there are few readings, one writer, one reader, and time is not critical.
Example: the office with one temperature reading per minute. Kafka would only be extra work there.

**Kafka is worth it** when one of these is true:

- many writers work at the same time and you cannot lose any reading.
- the rate reaches about a few hundred to a few thousand readings per second and results must be fresh - Database readers fall behind latest readings.
- more than one or two systems want to read the same data (archive, dashboard, detectors) - too many database accesses.
