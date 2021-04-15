package utils

func AddBits(sum int64, add int64) int64 {
	sum |= add
	return sum
}

func RemoveBits(sum int64, remove int64) int64 {
	sum &= ^remove
	return sum
}

func HasBits(sum int64, bit int64) bool {
	return (sum & bit) == bit
}
