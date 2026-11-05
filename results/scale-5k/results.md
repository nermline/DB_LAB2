# Benchmark results

Generated 2026-11-04 19:21 on darwin/arm64, 10 CPUs. PostgreSQL 18.6 (Homebrew) (shared_buffers 128MB, synchronous_commit on), MongoDB 9.0.4 (WiredTiger cache 8.1 GB, write concern j: true (journaled)).

## Dataset

| Table | Rows |
|---|---:|
| faculties | 8 |
| departments | 32 |
| teachers | 1920 |
| courses | 640 |
| student_groups | 256 |
| students | 5000 |
| enrollments | 172500 |
| grades | 945000 |

## Verification

- student record: 200 random students, identical in all 4 variants
- group debtors: 50 random groups, identical in PostgreSQL and MongoDB
- course statistics: all 8 closed semesters, identical in PostgreSQL and MongoDB

## Load time

| Target | Data | Keys and indexes |
|---|---:|---:|
| PostgreSQL tables (COPY) | 2.8 s | 1.8 s |
| PostgreSQL jsonb (COPY) | 5.3 s | - |
| MongoDB (insertMany) | 2.0 s | 0.3 s |

## Storage

| Storage | Size |
|---|---:|
| postgresql_tables_total | 138 MB |
| postgresql_jsonb_total | 47 MB |
| mongodb_storage | 75 MB |
| mongodb_indexes | 2 MB |
| mongodb_total | 76 MB |
| mongodb_uncompressed_bson | 203 MB |

## B1 student record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 2000 | 1.535 | 2.851 | 3.515 | 1.614 | 619 |  |
| PostgreSQL tables (json_agg) | 1 | 2000 | 3.191 | 5.709 | 6.903 | 3.326 | 301 |  |
| PostgreSQL jsonb | 1 | 2000 | 1.365 | 2.401 | 2.780 | 1.417 | 706 |  |
| MongoDB | 1 | 2000 | 0.416 | 0.644 | 0.744 | 0.426 | 2349 |  |

## B2 group debtors

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.060 | 0.130 | 0.153 | 0.070 | 14262 |  |
| MongoDB | 1 | 500 | 0.206 | 0.350 | 0.420 | 0.221 | 4515 |  |

## B3 course statistics

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 16 | 5.779 | 8.423 | 8.423 | 5.489 | 182 |  |
| MongoDB | 1 | 16 | 41.884 | 53.344 | 53.344 | 35.282 | 28 |  |

## B4a add grade (default durability)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.123 | 0.265 | 0.363 | 0.146 | 6862 | synchronous_commit=on, wal_sync_method=open_datasync |
| MongoDB | 1 | 500 | 4.198 | 5.969 | 7.420 | 4.516 | 221 | j: true (F_FULLFSYNC of the journal) |

## B4b add grade (async commit in both)

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 500 | 0.078 | 0.110 | 0.147 | 0.075 | 13235 | synchronous_commit=off |
| MongoDB | 1 | 500 | 0.102 | 0.159 | 0.257 | 0.107 | 9337 | w: 1, j: false |

## B5 rename course

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables | 1 | 30 | 0.056 | 0.093 | 0.142 | 0.059 | 16827 | rows/documents changed per call: 1 |
| MongoDB | 1 | 30 | 23.020 | 32.013 | 42.037 | 19.775 | 51 | rows/documents changed per call: 273 |

## B6 concurrent record

| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| PostgreSQL tables (JOIN) | 1 | 2634 | 1.819 | 3.269 | 3.996 | 1.898 | 527 |  |
| PostgreSQL tables (json_agg) | 1 | 1464 | 3.267 | 5.879 | 6.844 | 3.417 | 293 |  |
| PostgreSQL jsonb | 1 | 3840 | 1.260 | 2.239 | 2.572 | 1.302 | 768 |  |
| MongoDB | 1 | 12040 | 0.403 | 0.645 | 0.740 | 0.415 | 2408 |  |
| PostgreSQL tables (JOIN) | 8 | 26431 | 1.469 | 2.395 | 2.877 | 1.514 | 5285 |  |
| PostgreSQL tables (json_agg) | 8 | 20814 | 1.890 | 3.059 | 3.530 | 1.922 | 4160 |  |
| PostgreSQL jsonb | 8 | 43350 | 0.886 | 1.625 | 1.865 | 0.923 | 8668 |  |
| MongoDB | 8 | 50567 | 0.756 | 1.366 | 1.610 | 0.791 | 10112 |  |
| PostgreSQL tables (JOIN) | 32 | 27747 | 1.808 | 9.214 | 141.177 | 5.768 | 5543 |  |
| PostgreSQL tables (json_agg) | 32 | 21027 | 2.752 | 17.435 | 133.358 | 7.615 | 4199 |  |
| PostgreSQL jsonb | 32 | 47070 | 1.699 | 6.205 | 17.454 | 3.400 | 9408 |  |
| MongoDB | 32 | 59705 | 2.747 | 3.965 | 4.970 | 2.680 | 11934 |  |
| PostgreSQL tables (JOIN) | 64 | 26579 | 1.919 | 18.039 | 377.823 | 12.050 | 5304 |  |
| PostgreSQL tables (json_agg) | 64 | 18925 | 2.813 | 86.889 | 353.322 | 16.926 | 3778 |  |
| PostgreSQL jsonb | 64 | 48417 | 1.830 | 13.123 | 134.011 | 6.613 | 9671 |  |
| MongoDB | 64 | 64222 | 5.440 | 6.829 | 7.649 | 4.985 | 12832 |  |
