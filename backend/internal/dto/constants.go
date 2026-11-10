package dto

const MaxPageSize = 100

// MaxPage page 上界：钳制后 (page-1)*pageSize ≤ (MaxPage-1)*MaxPageSize，
// 远离 int 溢出（issue #11），同时 10 亿页已超出任何真实数据规模
const MaxPage = 1_000_000_000
