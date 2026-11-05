#!/usr/bin/env bash
# Requirement 6: basic queries of a NoSQL database of another type (Redis, key-value).
set -euo pipefail

CLI="${REDIS_CLI:-redis-cli}"
run() {
  local shown="" a
  for a in "$@"; do
    if [[ "$a" =~ [[:space:]] ]]; then shown+=" \"$a\""; else shown+=" $a"; fi
  done
  printf '\n>%s\n' "$shown"
  "$CLI" "$@"
}
section() { printf '\n==== %s ====\n' "$1"; }

"$CLI" --scan --pattern 'uni:*' | xargs -r "$CLI" del > /dev/null

section "1. String: cached student record with a time to live"
run SET uni:student:1:record '{"id":1,"name":"Kovalenko Khrystyna","group":"SE-24-01","rating":94.33}' EX 300
run GET uni:student:1:record
run TTL uni:student:1:record
run EXISTS uni:student:1:record uni:student:2:record

section "1. String: counters"
run INCR uni:stats:portal_views:2026-11-04
run INCRBY uni:stats:portal_views:2026-11-04 41
run GET uni:stats:portal_views:2026-11-04

section "1. String: login attempts limit (5 per minute)"
run SET uni:login_attempts:s000001 0 EX 60 NX
run INCR uni:login_attempts:s000001
run INCR uni:login_attempts:s000001
run TTL uni:login_attempts:s000001

section "2. Hash: student profile"
run HSET uni:student:1 record_book RB2024-000001 last_name Kovalenko first_name Khrystyna group SE-24-01 status active failed_courses 0
run HGET uni:student:1 group
run HMGET uni:student:1 last_name first_name status
run HINCRBY uni:student:1 failed_courses 1
run HSET uni:student:1 status academic_leave
run HGETALL uni:student:1
run HDEL uni:student:1 failed_courses
run HEXISTS uni:student:1 failed_courses

section "3. List: latest notifications of a student (newest first, keep 20)"
run LPUSH uni:student:1:notifications "Grade 9/10 for Lab 1 (Databases)"
run LPUSH uni:student:1:notifications "Monday lesson 2 moved to B-210"
run LPUSH uni:student:1:notifications "Scholarship assigned for 2026/2027"
run LRANGE uni:student:1:notifications 0 -1
run LTRIM uni:student:1:notifications 0 19
run LLEN uni:student:1:notifications

section "3. List as a queue: grade recalculation jobs (LPUSH + RPOP = FIFO)"
run LPUSH uni:jobs:recalc enrollment:1501 enrollment:1502 enrollment:1503
run RPOP uni:jobs:recalc
run RPOP uni:jobs:recalc
run LLEN uni:jobs:recalc

section "4. Set: students of courses"
run SADD uni:course:SE-301:students 1 2 3 4 5
run SADD uni:course:CS-201:students 4 5 6 7
run SISMEMBER uni:course:SE-301:students 3
run SCARD uni:course:SE-301:students
run SINTER uni:course:SE-301:students uni:course:CS-201:students
run SUNION uni:course:SE-301:students uni:course:CS-201:students
run SDIFF uni:course:SE-301:students uni:course:CS-201:students

section "5. Sorted set: group ranking (leaderboard)"
run ZADD uni:rating:SE-24-01 94.33 "Kovalenko Khrystyna" 92.78 "Humeniuk Viktoriia" 89.78 "Zakharchenko Taras" 82 "Yaroshenko Viktoriia" 71.5 "Holovko Olena"
run ZREVRANGE uni:rating:SE-24-01 0 2 WITHSCORES
run ZREVRANK uni:rating:SE-24-01 "Yaroshenko Viktoriia"
run ZINCRBY uni:rating:SE-24-01 9 "Yaroshenko Viktoriia"
run ZREVRANK uni:rating:SE-24-01 "Yaroshenko Viktoriia"
run ZRANGEBYSCORE uni:rating:SE-24-01 90 100 WITHSCORES
run ZCOUNT uni:rating:SE-24-01 "(75" +inf
run ZSCORE uni:rating:SE-24-01 "Holovko Olena"

section "6. Stream: event log of grade changes"
run XADD uni:events:grades '*' student 1 course SE-301 title "Lab 1" score 9
run XADD uni:events:grades '*' student 2 course SE-301 title "Lab 1" score 7
run XLEN uni:events:grades
run XRANGE uni:events:grades - + COUNT 10

section "7. Transaction: grade + ranking + notification together"
printf '\n> MULTI; ZINCRBY ...; LPUSH ...; HINCRBY ...; EXEC\n'
printf 'MULTI\nZINCRBY uni:rating:SE-24-01 5 "Holovko Olena"\nLPUSH uni:student:5:notifications "Exam: 38/40"\nHINCRBY uni:stats:grades total 1\nEXEC\n' | "$CLI"

section "8. Keys: search, type, expiration"
run SCAN 0 MATCH 'uni:student:*' COUNT 100
run TYPE uni:rating:SE-24-01
run EXPIRE uni:student:1:notifications 3600
run TTL uni:student:1:notifications
run PERSIST uni:student:1:notifications
run TTL uni:student:1:notifications
run DEL uni:jobs:recalc
run DBSIZE
