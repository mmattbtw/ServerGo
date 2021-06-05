local key = ARGV[1]
local expire = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local by = tonumber(ARGV[4])

local exists = redis.call("EXISTS", "rl2:" .. key)

local count = redis.call("INCRBY", "rl2:" .. key, by)

if exists == 0 then
    redis.call("EXPIRE", "rl2:" .. key, expire)
    return {count, expire}
end

local ttl = redis.call("TTL", "rl2:" .. key)

if count > limit then
    return {redis.call("DECRBY", "rl2:" .. key, by), ttl, 1}
end

return {count, ttl, 0}
