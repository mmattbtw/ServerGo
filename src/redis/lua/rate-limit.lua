local key = ARGV[1]
local expire = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local by = tonumber(ARGV[4])

local exists = redis.call("EXISTS", "rl:" .. key)

local count = redis.call("INCRBY", "rl:" .. key, by)

if exists == 0 then
    redis.call("EXPIRE", "rl:" .. key, expire)
    return {count, expire}
end

local ttl = redis.call("TTL", "rl:" .. key)

if count > limit then
    return {redis.call("DECRBY", "rl:" .. key, by), ttl, 1}
end

return {count, ttl, 0}
