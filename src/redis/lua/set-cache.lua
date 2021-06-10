-- TimeComplexity O(n) where n is the number of object IDs (length/ARGV[2])
-- SpaceComplexity O(n) where n is the number of object IDs (length/ARGV[2])
local queryKey = KEYS[1]
local objectKey = KEYS[2]
local ciKey = KEYS[3]

local now = ARGV[1]
local sha = ARGV[2]
local length = tonumber(ARGV[3])

local queryString = ""
local ojson
local oid
local c

for i=1,length/2,1 do
	oid = ARGV[i*2+2]
	ojson = ARGV[i*2+3]
	redis.call("SET", objectKey .. ":" .. oid, ojson, "EX", 600)
	redis.call('ZADD', objectKey, now, oid)
	if i ~= 1 then 
		queryString = queryString .. " " .. oid
	else
		queryString = oid
	end
end

local clearBefore = now - 600

redis.call('ZREMRANGEBYSCORE', queryKey, 0, clearBefore)
redis.call('ZREMRANGEBYSCORE', objectKey, 0, clearBefore)
redis.call("EXPIRE", objectKey, 600)
if ciKey ~= nil then
	redis.call('ZREMRANGEBYSCORE', ciKey, 0, clearBefore)
	redis.call('ZADD', ciKey, now, sha)
	redis.call("EXPIRE", ciKey, 600)
end

redis.call("SET", queryKey .. ":" .. sha, queryString, "EX", 600)
redis.call('ZADD', queryKey, now, sha)
redis.call("EXPIRE", queryKey, 600)

return 1
