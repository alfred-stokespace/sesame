# Sesame
Make simple things simple again for AWS simple systems manager (SSM)

SSM is a powerful product offering. However, it makes some tasks very difficult. 
## Why so difficult SSM?
1. Much of what SSM does is asyncronous, so you have to sit and poll for results.
2. Much of what SSM does fans out to many hosts. so you have to navigate one-to-many relationships (some of which are then asyncronous)
3. SSM API's are not great about dealing with aliases that mean something to you or your customer. Some API's allow tag queries and some don't.  
4. The Console can be alot to learn and navigate for someone that just wants to operate on a single host they care about. 

Tasks that should be simplerererrrr...
1. I know my on-prem (or EC2) host by a nickname, I want to operate on it by using it's nickname.
    2. ```
       export AWS_PROFILE=your-profile
       export AWS_REGION=us-west-2
       
       go run cmd/sesame/main.go search -n DrStrange 2> /dev/null
       mi-01d856ea25bf2f111
       ``` 
3. I issued an operation (run, automation) against a tag set filter, how did it go for a host I know by nickname? 
4. 
