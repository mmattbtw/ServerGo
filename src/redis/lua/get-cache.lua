-- TimeComplexity O(n) where n is the number of object IDs in that query.
-- SpaceComplexity O(n) where n is the number of object IDs in that query. 
local queryKey = KEYS[1]
local objectKey = KEYS[2]
local sha = ARGV[1]

local objectIDs = redis.call("GET", queryKey .. ":" .. sha)

if objectIDs == false then
	return nil
end

local items = ""
local missingItems = {}
for id in string.gmatch(objectIDs, "[^%s]+") do
	local item = redis.call("GET", objectKey .. ":" .. id)
	if item == nil then
		missingItems[#missingItems + 1] = id
	else
		if items == "" then
			items = item
		else
			items = items .. "," .. item
		end
	end
end

if #items == 0 then
	return nil
end

return {"["..items.."]", missingItems}
