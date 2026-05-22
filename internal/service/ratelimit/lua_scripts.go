package ratelimit

import "github.com/redis/go-redis/v9"

// ---- Lua scripts for atomic Redis operations ----

// luaCheckAndReserve atomically checks if adding estimatedCost would exceed
// the limit and, if not, reserves that amount.
//
// KEYS[1] = counter key (e.g. "ratelimit:amount:daily:42")
// ARGV[1] = estimated cost (float string)
// ARGV[2] = limit amount (float string)
// ARGV[3] = TTL in seconds (integer string); 0 means no expiry
//
// Returns:
//
//	{1, newTotal}   -- success: reservation made
//	{0, currentTotal} -- failure: would exceed limit
var luaCheckAndReserve = redis.NewScript(`
local key   = KEYS[1]
local cost  = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ttl   = tonumber(ARGV[3])

local current = tonumber(redis.call('GET', key) or '0') or 0

if current + cost > limit then
  return {0, tostring(current)}
end

local newTotal = redis.call('INCRBYFLOAT', key, cost)

if ttl > 0 then
  local existing = redis.call('TTL', key)
  if existing < 0 then
    redis.call('EXPIRE', key, ttl)
  end
end

return {1, newTotal}
`)

// luaAdjustCounter atomically adjusts a counter by a delta (can be negative).
//
// KEYS[1] = counter key
// ARGV[1] = delta (float string, can be negative)
//
// Returns: new counter value as string
var luaAdjustCounter = redis.NewScript(`
local key   = KEYS[1]
local delta = tonumber(ARGV[1])

local newVal = redis.call('INCRBYFLOAT', key, delta)
local numVal = tonumber(newVal)

-- Clamp to zero: don't allow negative counters
if numVal < 0 then
  redis.call('SET', key, '0')
  -- Preserve TTL
  local ttl = redis.call('TTL', key)
  if ttl > 0 then
    redis.call('EXPIRE', key, ttl)
  end
  return '0'
end

return newVal
`)

// luaDecrementBudgetLease atomically decrements the remainingBudget in a
// cached BudgetLease JSON blob.
//
// KEYS[1] = lease key (e.g. "lease:user:42:daily")
// ARGV[1] = cost to decrement (float string)
//
// Returns:
//
//	{newRemaining, 1}  -- success
//	{0, 0}             -- insufficient budget
//	{-1, 0}            -- key not found
var luaDecrementBudgetLease = redis.NewScript(`
local key  = KEYS[1]
local cost = tonumber(ARGV[1])

local leaseJson = redis.call('GET', key)
if not leaseJson then
  return {'-1', '0'}
end

local lease = cjson.decode(leaseJson)
local remaining = tonumber(lease.remainingBudget) or 0

if remaining < cost then
  return {'0', '0'}
end

local newRemaining = remaining - cost
lease.remainingBudget = newRemaining

local ttl = redis.call('TTL', key)
if ttl > 0 then
  redis.call('SETEX', key, ttl, cjson.encode(lease))
else
  redis.call('SET', key, cjson.encode(lease))
end

return {tostring(newRemaining), '1'}
`)
