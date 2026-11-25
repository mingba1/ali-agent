package bootstrap

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/service"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS doc (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		title VARCHAR(200) NOT NULL,
		content MEDIUMTEXT NOT NULL,
		tags VARCHAR(255) NOT NULL DEFAULT '',
		author VARCHAR(64) NOT NULL DEFAULT 'system',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_doc_title (title)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS doc_chunk (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		doc_id BIGINT UNSIGNED NOT NULL,
		chunk_index INT NOT NULL DEFAULT 0,
		heading VARCHAR(255) NOT NULL DEFAULT '',
		content TEXT NOT NULL,
		start_offset INT NOT NULL DEFAULT 0,
		end_offset INT NOT NULL DEFAULT 0,
		embedding MEDIUMTEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_chunk_doc (doc_id, chunk_index)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS agent (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		name VARCHAR(64) NOT NULL,
		role VARCHAR(64) NOT NULL DEFAULT '',
		skills VARCHAR(255) NOT NULL DEFAULT '',
		workload INT NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'idle',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY uk_agent_name (name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS task (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		title VARCHAR(200) NOT NULL,
		content TEXT,
		assignee VARCHAR(64) NOT NULL DEFAULT '',
		status VARCHAR(20) NOT NULL DEFAULT 'todo',
		priority VARCHAR(16) NOT NULL DEFAULT 'medium',
		due_date DATE DEFAULT NULL,
		creator VARCHAR(64) NOT NULL DEFAULT 'system',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_task_status (status),
		KEY idx_task_assignee (assignee)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS chat_session (
		id VARCHAR(64) NOT NULL,
		user_id VARCHAR(64) NOT NULL DEFAULT 'default',
		title VARCHAR(200) NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_session_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

	`CREATE TABLE IF NOT EXISTS chat_message (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		session_id VARCHAR(64) NOT NULL,
		role VARCHAR(16) NOT NULL,
		content MEDIUMTEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		KEY idx_message_session (session_id, id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}

func Init(ctx context.Context) error {
	if err := g.DB().PingMaster(); err != nil {
		return gerror.Wrap(err, "MySQL 连接失败，请先启动数据库并创建 ali_agent 库")
	}
	for _, stmt := range schema {
		if _, err := g.DB().Exec(ctx, stmt); err != nil {
			return gerror.Wrap(err, "初始化表结构失败")
		}
	}
	if err := seed(ctx); err != nil {
		return err
	}
	if service.Doc() != nil {
		docs, chunks, err := service.Doc().Reindex(ctx)
		if err != nil {
			return gerror.Wrap(err, "构建文档分块与向量索引失败")
		}
		g.Log().Infof(ctx, "doc index ensured: docs=%d chunks=%d", docs, chunks)
	}
	g.Log().Info(ctx, "ali-agent bootstrap completed")
	return nil
}

func seed(ctx context.Context) error {
	count, err := g.Model("doc").Ctx(ctx).Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	list := g.List{
		g.Map{
			"title": "差旅报销流程",
			"tags":  "报销,财务,流程,差旅",
			"content": "员工出差前需在系统提交出差申请，经直属主管审批后方可预订行程。" +
				"出差结束后 7 个工作日内提交报销单，附发票、行程单和审批记录。" +
				"市内交通与住宿按公司差旅标准执行，超标部分需部门负责人单独审批。" +
				"财务每周三、周五集中打款，报销单状态可在「我的报销」中查看。",
			"author": "财务部",
		},
		g.Map{
			"title": "请假与考勤制度",
			"tags":  "请假,考勤,人事,制度",
			"content": "年假按入职年限计算，满一年 5 天，满三年 10 天，满五年 15 天。" +
				"请假需提前在 OA 提交申请：1 天以内由直属主管审批，超过 3 天需部门负责人审批。" +
				"病假需提供医院证明，事假按日扣除当日工资。" +
				"每月迟到超过 3 次将影响当季绩效考核，弹性打卡时间为 9:00-10:00。",
			"author": "人力资源部",
		},
		g.Map{
			"title": "项目排期与任务协作规范",
			"tags":  "项目,排期,任务,协作,敏捷",
			"content": "项目启动后由产品经理输出 PRD 与里程碑排期，研发负责人拆解任务并录入协同办公系统。" +
				"任务状态统一为 todo、doing、review、done 四态，每日站会同步阻塞项。" +
				"优先级分为 low、medium、high、urgent，urgent 任务需在 24 小时内响应。" +
				"任务完成后由测试 Agent 验证，通过后流转为 done，并在周报中汇总交付情况。",
			"author": "项目管理办公室",
		},
	}
	_, err = g.Model("doc").Ctx(ctx).Data(list).Insert()
	return err
}
