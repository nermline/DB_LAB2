# Benchmark results

Generated 2026-11-04 19:26 on darwin/arm64, 10 CPUs. PostgreSQL 18.6 (Homebrew) (shared_buffers 128MB, synchronous_commit on), MongoDB 9.0.4 (WiredTiger cache 8.1 GB, write concern j: true (journaled)).

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

## Storage

| Storage | Size |
|---|---:|
| postgresql_tables_total | 1371 MB |
| postgresql_jsonb_total | 472 MB |
| mongodb_storage | 734 MB |
| mongodb_indexes | 15 MB |
| mongodb_total | 749 MB |
| mongodb_uncompressed_bson | 2034 MB |

## B1 student record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 2000 | 2.164 | 3.679 | 4.399 | 2.254 | 444 |  |
| PostgreSQL tables (json_agg) | 1 | 2000 | 3.599 | 5.944 | 6.865 | 3.697 | 270 |  |
| PostgreSQL jsonb | 1 | 2000 | 1.590 | 2.669 | 3.039 | 1.621 | 617 |  |
| MongoDB | 1 | 2000 | 0.390 | 0.555 | 0.652 | 0.384 | 2605 |  |

## B2 group debtors

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.070 | 0.126 | 0.173 | 0.075 | 13283 |  |
| MongoDB | 1 | 500 | 0.267 | 0.353 | 0.386 | 0.268 | 3733 |  |

## B3 course statistics

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 16 | 16.374 | 27.372 | 27.372 | 16.982 | 59 |  |
| MongoDB | 1 | 16 | 406.514 | 511.560 | 511.560 | 338.555 | 3 |  |

## B4a add grade (default durability)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.478 | 0.741 | 0.837 | 0.489 | 2044 | synchronous_commit=on, wal_sync_method=open_datasync |
| MongoDB | 1 | 500 | 4.993 | 6.130 | 7.051 | 5.015 | 199 | j: true (F_FULLFSYNC of the journal) |

## B4b add grade (async commit in both)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.056 | 0.080 | 0.106 | 0.056 | 17995 | synchronous_commit=off |
| MongoDB | 1 | 500 | 0.101 | 0.136 | 0.157 | 0.103 | 9719 | w: 1, j: false |

## B5 rename course

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 30 | 0.098 | 0.257 | 0.289 | 0.129 | 7775 | rows/documents changed per call: 1 |
| MongoDB | 1 | 30 | 199.910 | 291.271 | 368.867 | 174.029 | 6 | rows/documents changed per call: 2734 |

## B6 concurrent record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 1776 | 2.760 | 4.449 | 4.986 | 2.817 | 355 |  |
| PostgreSQL tables (json_agg) | 1 | 1353 | 3.603 | 5.955 | 6.899 | 3.699 | 270 |  |
| PostgreSQL jsonb | 1 | 3006 | 1.574 | 2.742 | 3.140 | 1.664 | 601 |  |
| MongoDB | 1 | 11412 | 0.428 | 0.668 | 0.826 | 0.438 | 2282 |  |
| PostgreSQL tables (JOIN) | 8 | 27946 | 1.344 | 2.377 | 3.134 | 1.432 | 5586 |  |
| PostgreSQL tables (json_agg) | 8 | 19322 | 1.987 | 3.176 | 3.854 | 2.071 | 3863 |  |
| PostgreSQL jsonb | 8 | 43637 | 0.885 | 1.615 | 1.945 | 0.917 | 8725 |  |
| MongoDB | 8 | 60686 | 0.589 | 1.237 | 1.464 | 0.659 | 12136 |  |
| PostgreSQL tables (JOIN) | 32 | 30457 | 1.628 | 4.918 | 148.660 | 5.254 | 6087 |  |
| PostgreSQL tables (json_agg) | 32 | 22431 | 2.517 | 26.812 | 119.567 | 7.137 | 4481 |  |
| PostgreSQL jsonb | 32 | 53276 | 1.117 | 4.653 | 43.242 | 3.004 | 10648 |  |
| MongoDB | 32 | 66740 | 2.462 | 3.625 | 4.166 | 2.398 | 13343 |  |
| PostgreSQL tables (JOIN) | 64 | 26859 | 1.785 | 14.751 | 384.496 | 11.923 | 5361 |  |
| PostgreSQL tables (json_agg) | 64 | 18432 | 2.921 | 89.636 | 324.236 | 17.388 | 3675 |  |
| PostgreSQL jsonb | 64 | 49887 | 1.245 | 8.887 | 146.685 | 6.420 | 9961 |  |
| MongoDB | 64 | 64292 | 5.327 | 6.955 | 8.399 | 4.980 | 12847 |  |
