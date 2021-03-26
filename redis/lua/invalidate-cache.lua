-- TimeComplexity O(n*m) where n is the number of cached queries and m is the average number of ids in each document. 
-- SpaceComplexity O(1)
local invalidateKey = KEYS[1]
local queryKey = KEYS[2]
local objectKey = KEYS[3]
local commonIndexKey = KEYS[4]
local now = ARGV[1]
local oid = ARGV[2]
local ojson = ARGV[3]

if invalidateKey ~= "" then
    if redis.call("SET", invalidateKey, 1, "NX", "EX", 3600) == nil then
        return 0
    end
end

local clearBefore = now - 3600
redis.call('ZREMRANGEBYSCORE', queryKey, 0, clearBefore)

local queries = redis.call("ZRANGE", queryKey, 0, -1)
local value
local key
for i=1,#queries do
    key = queryKey .. ":" .. queries[i]
    value = redis.call("GET", key)
    if value ~= nil and value ~= false and string.find(value, oid) then
        redis.call("DEL", key)
        redis.call("ZREM", queryKey, queries[i])
    end
end

if ojson == "" then
    redis.call("DEL", objectKey .. ":" .. oid)
    redis.call("ZREM", objectKey, oid)
else
    redis.call("SET", objectKey .. ":" .. oid, ojson, "EX", 3600)
    redis.call("ZADD", objectKey, now, oid)
end

if commonIndexKey ~= nil then
    redis.call('ZREMRANGEBYSCORE', commonIndexKey, 0, clearBefore)

    local queries = redis.call("ZRANGE", commonIndexKey, 0, -1)
    local value
    local key
    for i=1,#queries do
        key = objectKey .. ":" .. queries[i]
        value = redis.call("GET", key)
        if val ~= nil and string.find(value, oid) then
            redis.call("DEL", key)
            redis.call("ZREM", queryKey, queries[i])
        end
    end
end

return 1
