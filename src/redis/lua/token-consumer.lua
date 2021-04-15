local token = ARGV[1]

local userID = tostring(redis.call("GET", "temp:codes:" .. token))

if not userID then
	return nil
end

redis.call("DEL", "temp:codes:" .. token)

return userID
