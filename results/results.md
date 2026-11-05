# Benchmark results

Generated 2026-11-04 19:19 on darwin/arm64, 10 CPUs. PostgreSQL 18.6 (Homebrew) (shared_buffers 128MB, synchronous_commit on), MongoDB 9.0.4 (WiredTiger cache 8.1 GB, write concern j: true (journaled)).

## Dataset

| Table | Rows |
|---|---:|
| faculties | 8 |
| departments | 32 |
| teachers | 1920 |
| courses | 640 |
| student_groups | 2048 |
| students | 50000 |
| enrollments | 1725000 |
| grades | 9450000 |

## Verification

- student record: 200 random students, identical in all 4 variants
- group debtors: 50 random groups, identical in PostgreSQL and MongoDB
- course statistics: all 8 closed semesters, identical in PostgreSQL and MongoDB

## Load time

| Target | Data | Keys and indexes |
|---|---:|---:|
| PostgreSQL tables (COPY) | 29.2 s | 22.8 s |
| PostgreSQL jsonb (COPY) | 54.6 s | - |
| MongoDB (insertMany) | 19.4 s | 2.9 s |

## Storage

| Storage | Size |
|---|---:|
| postgresql_tables_total | 1371 MB |
| postgresql_jsonb_total | 472 MB |
| mongodb_storage | 730 MB |
| mongodb_indexes | 15 MB |
| mongodb_total | 745 MB |
| mongodb_uncompressed_bson | 2034 MB |

## B1 student record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 2000 | 2.068 | 3.688 | 5.831 | 2.284 | 438 |  |
| PostgreSQL tables (json_agg) | 1 | 2000 | 3.642 | 6.143 | 7.029 | 3.774 | 265 |  |
| PostgreSQL jsonb | 1 | 2000 | 1.575 | 2.676 | 2.994 | 1.617 | 618 |  |
| MongoDB | 1 | 2000 | 0.472 | 0.667 | 0.781 | 0.482 | 2072 |  |

## B2 group debtors

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.079 | 0.154 | 0.270 | 0.089 | 11216 |  |
| MongoDB | 1 | 500 | 0.358 | 0.563 | 0.653 | 0.373 | 2683 |  |

## B3 course statistics

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 16 | 16.730 | 26.057 | 26.057 | 17.650 | 57 |  |
| MongoDB | 1 | 16 | 405.671 | 515.790 | 515.790 | 348.975 | 3 |  |

## B4a add grade (default durability)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.386 | 0.631 | 0.790 | 0.396 | 2527 | synchronous_commit=on, wal_sync_method=open_datasync |
| MongoDB | 1 | 500 | 4.994 | 6.144 | 16.759 | 5.247 | 191 | j: true (F_FULLFSYNC of the journal) |

## B4b add grade (async commit in both)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.064 | 0.078 | 0.107 | 0.062 | 16198 | synchronous_commit=off |
| MongoDB | 1 | 500 | 0.100 | 0.137 | 0.183 | 0.101 | 9883 | w: 1, j: false |

## B5 rename course

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 30 | 0.117 | 0.479 | 0.521 | 0.183 | 5461 | rows/documents changed per call: 1 |
| MongoDB | 1 | 30 | 176.875 | 315.439 | 356.733 | 165.797 | 6 | rows/documents changed per call: 2734 |

## B6 concurrent record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 1616 | 2.775 | 5.440 | 10.637 | 3.094 | 323 |  |
| PostgreSQL tables (json_agg) | 1 | 1332 | 3.655 | 6.135 | 7.120 | 3.758 | 266 |  |
| PostgreSQL jsonb | 1 | 3157 | 1.511 | 2.764 | 3.503 | 1.584 | 631 |  |
| MongoDB | 1 | 11018 | 0.439 | 0.685 | 0.874 | 0.454 | 2204 |  |
| PostgreSQL tables (JOIN) | 8 | 28093 | 1.362 | 2.307 | 2.770 | 1.425 | 5614 |  |
| PostgreSQL tables (json_agg) | 8 | 20571 | 1.890 | 3.009 | 3.466 | 1.945 | 4112 |  |
| PostgreSQL jsonb | 8 | 45528 | 0.845 | 1.538 | 1.878 | 0.879 | 9104 |  |
| MongoDB | 8 | 61513 | 0.581 | 1.224 | 1.453 | 0.650 | 12301 |  |
| PostgreSQL tables (JOIN) | 32 | 29918 | 1.676 | 5.183 | 146.907 | 5.349 | 5978 |  |
| PostgreSQL tables (json_agg) | 32 | 20306 | 2.748 | 22.567 | 144.829 | 7.884 | 4056 |  |
| PostgreSQL jsonb | 32 | 50506 | 1.144 | 4.851 | 58.124 | 3.169 | 10095 |  |
| MongoDB | 32 | 70400 | 2.331 | 3.495 | 4.011 | 2.273 | 14073 |  |
| PostgreSQL tables (JOIN) | 64 | 28279 | 1.644 | 9.425 | 483.345 | 11.324 | 5645 |  |
| PostgreSQL tables (json_agg) | 64 | 21360 | 3.126 | 75.145 | 180.167 | 15.000 | 4262 |  |
| PostgreSQL jsonb | 64 | 50539 | 1.195 | 10.796 | 135.199 | 6.335 | 10097 |  |
| MongoDB | 64 | 68793 | 5.038 | 6.617 | 8.328 | 4.653 | 13747 |  |
