-- TimeComplexity O(n*m) where n is the number of cached queries and m is the average number of ids in each document. 
-- SpaceComplexity O(1)
local queryKey = KEYS[1]
local commonIndexKey = KEYS[2]
local now = ARGV[1]

local clearBefore = now - 600
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

return 1
