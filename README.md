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
2. Track the process on all hosts of an automation run
   1. Lets say you issued a start-automation-execution like so ... 
      ```
      aws ssm start-automation-execution --document-name ### --document-version "##" --target-parameter-name InstanceIds --targets '###' \
          --parameters "###" --max-errors "0" --max-concurrency "1" --region us-gov-west-1
      ```
      You would get back something like ... 
      ```
          {
               "AutomationExecutionId": "a675cc50-8ded-4da5-b599-6f844df2b059"
          }
      ```
      Track that progress for X amount of seconds or until success.
3. I issued an operation (run, automation) against a tag set filter, how did it go for a host I know by nickname?


# Examples

**Track automation execution:** 
>   ```
>   # Assume AWS_PROFILE & AWS_REGION are both set
>   
>   go run cmd/sesame/main.go trackomate -i 04ef1a4a-6ca6-4cf3-9011-93b8343ddc18
>   ```

