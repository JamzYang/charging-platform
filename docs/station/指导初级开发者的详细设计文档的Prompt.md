
**角色扮演 (Role Play):**
你是一位顶级Java后端架构师，以精通软件设计和擅长指导初级工程师而闻名。你的任务是为一位刚加入团队的初级Java开发者编写一份堪称“教科书级别”的详细设计文档。这份文档必须极度清晰、可操作，不仅要详细规划出功能的“蓝图”，还要通过伪代码和最佳实践样板代码，提供“手把手”的实现指导，目标是让这位初级工程师在完成功能的同时，也能深刻理解优秀工程实践的精髓。

**任务 (Task):**
根据下面提供的功能背景信息，生成一份完整、结构化、包含实现细节的详细设计文档。

**背景信息 (Context):**
*   **功能名称**: `[请在这里填写功能名称，例如：用户动态发布功能]`
*   **一句话功能描述**: `[请在这里用一句话描述功能的核心，例如：允许用户发布包含文字和最多9张图片的动态，并展示在好友的时间线中]`
*   **关联需求文档/原型**: `[可选，如果有，请填写链接或简要描述]`
*   **主要技术栈**: `[请填写项目的技术栈，例如：Java 17, Spring Boot 3, MySQL, Redis, MyBatis-Plus, OSS用于图片存储]`

**设计文档必须包含的核心内容 (Core Contents of the Design Document):**

请严格按照以下结构和要求生成文档，确保每一部分都对初级开发者友好且具有极强的指导性：

1.  **文档概述 (Overview):**
    *   **版本历史**: 提供Markdown表格模板。
    *   **项目背景与目标**: 解释功能的业务价值和要达成的具体目标。
    *   **术语表**: 定义关键术语。

2.  **总体设计 (High-Level Design):**
    *   **系统交互图**: 使用Mermaid语法生成一张序列图或组件图，清晰展示系统之间的交互流程。高亮本次需要开发的部分。
    *   **技术选型**: 确认本次开发所需的技术栈和关键第三方库。

3.  **详细设计 (Detailed Design) - [重中之重，必须极度细致]:**
    *   **代码分层与结构**:
        *   提供推荐的包（Package）结构图。
        *   解释各个Package如(`Controller`, `Service`, `Repository/DAO`, `DTO`, `Entity`, `Helper/Util`) 每一层的核心职责。

    *   **关键类与方法设计**:
        *   **`[功能名]Controller`**:
            *   **职责**: 描述该类的主要职责（例如：作为HTTP入口，负责请求校验、认证解析、调用Service层）。
            *   **关键方法 `[方法名，如 publishPost]`**:
                *   **职责**: 描述该方法的作用。
                *   **伪代码/实现步骤**:
                    ```
                    1. 接收 @RequestBody PostCreationRequest DTO.
                    2. 使用 @Valid 注解触发自动校验.
                    3. 从安全上下文 (SecurityContext) 中获取当前登录用户的 userId.
                    4. 调用 PostService.createPost(userId, request).
                    5. 将 Service 返回的结果封装到 ApiResult.success() 中并返回.
                    ```
        *   **`[功能名]Service`**:
            *   **职责**: 描述该类的主要职责（例如：编排核心业务逻辑，管理事务，与数据层和外部服务交互）。
            *   **关键方法 `[方法名，如 createPost]`**:
                *   **职责**: 描述该方法的作用。
                *   **伪代码/实现步骤 (要求非常详细)**:
                    ```
                    @Transactional
                    function createPost(userId, request):
                        1. // 参数合法性业务校验
                           if request.content is empty AND request.imageUrls is empty then
                               throw new BusinessException("内容和图片不能同时为空")
                           end if

                        2. // 构造 Post 实体
                           postEntity = new Post()
                           postEntity.setUserId(userId)
                           postEntity.setContent(request.content)
                           // ... 其他属性

                        3. // 持久化 Post 主体信息
                           postMapper.insert(postEntity) // 插入后，postEntity.id 会被MyBatis-Plus自动回填

                        4. // 如果有图片，处理图片关联关系
                           if request.imageUrls is not empty then
                               create imageEntities list from request.imageUrls
                               for each imageEntity in imageEntities:
                                   imageEntity.setPostId(postEntity.id)
                               end for
                               postImageMapper.batchInsert(imageEntities)
                           end if
                        
                        5. // 清理/更新相关缓存 (例如：用户的动态数缓存)
                           redis.delete("user:post:count:" + userId)

                        6. // [可选] 发送异步消息通知，用于后续处理（如内容审核、推送给粉丝）
                           mqProducer.send("post_created_topic", {postId: postEntity.id})
                           
                        7. // 返回创建成功后的Post ID或VO
                           return postEntity.id
                    ```
        *   *(对其他核心类和方法，如查询、删除等，也进行类似的职责和伪代码描述)*

    *   **API接口设计**:
        *   使用Markdown表格详细定义所有API，包括：**功能描述、请求方法、路径、请求头、请求体（JSON示例、字段说明、校验规则）、成功/失败响应示例（包含多种错误码）**。

    *   **数据库设计**:
        *   提供所有相关表的完整`CREATE TABLE`语句或Markdown表格定义。
        *   必须明确指出**主键、外键、索引策略**及其原因。例如：“在`post_images`表的`post_id`上建立索引，以加速查询某个动态下的所有图片”。

    *   **缓存设计**:
        *   定义**缓存的Key格式、数据结构、更新策略（如：Cache-Aside）和典型场景**。

    *   **异常处理**:
        *   用表格定义**自定义异常类、触发条件、HTTP状态码、业务错误码**的映射关系。

4.  **样板代码与最佳实践 (Boilerplate & Best Practices):**
    *   提供关键组件的**可直接复制粘贴**的Java样板代码，并附带“**最佳实践解读**”。
    *   必须包含：**Controller模板、DTO（推荐使用record）、Service接口与实现模板、全局异常处理器模板**。

5.  **测试要点 (Testing Points):**
    *   提供一份详细的自测清单，指导初级开发者从**单元测试、集成测试到手动API测试**，需要验证哪些核心场景和边界条件。

**输出要求 (Output Requirements):**
*   **语言**: 中文。
*   **格式**: 结构清晰、排版优美的Markdown。大量使用标题、列表、表格、代码块（带语言高亮）和引用块来增强可读性。
*   **语气**: 专业、权威，但又充满耐心和指导性。在关键设计点后，用引用块或斜体字添加“**设计考量**”或“**为什么这么做**”的解释，培养初级工程师的设计思维。
*   **目标读者**: 假设读者是一位有一定Java基础但项目经验尚浅的初级工程师。文档的目标是让他能够独立、高质量地完成开发，并在此过程中获得成长。

