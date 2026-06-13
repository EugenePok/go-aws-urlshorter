output "vpc_id" {
  description = "ID of VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnet IDs (ALB lives here)"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "Private subnet IDs (ECS/RDS/Redis live here)"
  value       = aws_subnet.private[*].id
}

output "availability_zones" {
  description = "AZs the subnets span"
  value       = local.azs
}

output "alb_security_group_id" {
  description = "Security group for the ALB"
  value       = aws_security_group.alb.id
}

output "ecs_security_group_id" {
  description = "Security group for ECS tasks"
  value       = aws_security_group.ecs.id
}

output "rds_security_group_id" {
  description = "Security group for RDS MySQL"
  value       = aws_security_group.rds.id
}

output "redis_security_group_id" {
  description = "Security group for ElastiCache Redis"
  value       = aws_security_group.redis.id
}
