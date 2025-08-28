// API配置
const API_BASE = '/api';

// 工具函数
const utils = {
    // 获取token
    getToken() {
        return localStorage.getItem('token');
    },

    // 设置token
    setToken(token) {
        localStorage.setItem('token', token);
    },

    // 清除token
    clearToken() {
        localStorage.removeItem('token');
    },

    // 获取用户信息
    getUser() {
        const userStr = localStorage.getItem('user');
        return userStr ? JSON.parse(userStr) : null;
    },

    // 设置用户信息
    setUser(user) {
        localStorage.setItem('user', JSON.stringify(user));
    },

    // 清除用户信息
    clearUser() {
        localStorage.removeItem('user');
    },

    // 检查是否登录
    isLoggedIn() {
        return !!this.getToken();
    },

    // 格式化日期
    formatDate(date) {
        return new Date(date).toLocaleString('zh-CN');
    },

    // 显示提示消息
    showMessage(message, type = 'info') {
        const alertDiv = document.createElement('div');
        alertDiv.className = `alert alert-${type}`;
        alertDiv.textContent = message;
        
        const container = document.getElementById('alert-container') || document.body;
        container.insertBefore(alertDiv, container.firstChild);
        
        setTimeout(() => {
            alertDiv.remove();
        }, 3000);
    }
};

// API请求封装
const api = {
    // 基础请求方法
    async request(url, options = {}) {
        const token = utils.getToken();
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers
        };

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        try {
            const response = await fetch(`${API_BASE}${url}`, {
                ...options,
                headers
            });

            const data = await response.json();

            if (!response.ok) {
                if (response.status === 401) {
                    utils.clearToken();
                    utils.clearUser();
                    // 只在非登录页面时才重定向
                    const currentPath = window.location.pathname;
                    if (currentPath !== '/login' && currentPath !== '/register') {
                        window.location.href = '/login';
                    }
                }
                throw new Error(data.message || 'Request failed');
            }

            return data;
        } catch (error) {
            console.error('API request error:', error);
            throw error;
        }
    },

    // 认证相关
    auth: {
        async login(email, password) {
            const data = await api.request('/auth/login', {
                method: 'POST',
                body: JSON.stringify({ email, password })
            });
            
            if (data.success) {
                utils.setToken(data.data.token);
                utils.setUser(data.data.user);
            }
            
            return data;
        },

        async register(name, email, password) {
            return await api.request('/auth/register', {
                method: 'POST',
                body: JSON.stringify({ name, email, password })
            });
        },

        async logout() {
            await api.request('/auth/logout', { method: 'POST' });
            utils.clearToken();
            utils.clearUser();
            window.location.href = '/login';
        },

        async getProfile() {
            return await api.request('/auth/profile');
        }
    },

    // 知识库相关
    knowledgeBase: {
        async list(page = 1, pageSize = 10) {
            return await api.request(`/knowledge-bases?page=${page}&page_size=${pageSize}`);
        },

        async get(id) {
            return await api.request(`/knowledge-bases/${id}`);
        },

        async create(name, description) {
            return await api.request('/knowledge-bases', {
                method: 'POST',
                body: JSON.stringify({ name, description })
            });
        },

        async update(id, data) {
            return await api.request(`/knowledge-bases/${id}`, {
                method: 'PUT',
                body: JSON.stringify(data)
            });
        },

        async delete(id) {
            return await api.request(`/knowledge-bases/${id}`, {
                method: 'DELETE'
            });
        }
    },

    // 文档相关
    document: {
        async upload(file, kbId) {
            const formData = new FormData();
            formData.append('file', file);
            formData.append('kb_id', kbId);

            const token = utils.getToken();
            const response = await fetch(`${API_BASE}/documents/upload`, {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${token}`
                },
                body: formData
            });

            return await response.json();
        },

        async search(query, kbId, topK = 5) {
            return await api.request('/documents/search', {
                method: 'POST',
                body: JSON.stringify({
                    query,
                    kb_id: kbId ? parseInt(kbId) : undefined,
                    top_k: topK,
                    return_context: true
                })
            });
        },

        async list(kbId, page = 1, pageSize = 10) {
            return await api.request(`/knowledge-bases/${kbId}/documents?page=${page}&page_size=${pageSize}`);
        },

        async listAll(page = 1, pageSize = 10) {
            return await api.request(`/documents?page=${page}&page_size=${pageSize}`);
        },

        async delete(id) {
            return await api.request(`/documents/${id}`, {
                method: 'DELETE'
            });
        }
    },

    // 聊天相关
    chat: {
        async send(message, conversationId, kbId, useRAG = true) {
            return await api.request('/chat', {
                method: 'POST',
                body: JSON.stringify({
                    message,
                    conversation_id: conversationId,
                    kb_id: kbId ? parseInt(kbId) : undefined,
                    use_rag: useRAG
                })
            });
        },

        // 流式聊天
        async sendStream(message, conversationId, kbId, useRAG = true, onMessage, onError, onComplete, onContext) {
            const token = utils.getToken();
            
            try {
                const response = await fetch(`${API_BASE}/chat/stream`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${token}`
                    },
                    body: JSON.stringify({
                        message,
                        conversation_id: conversationId,
                        kb_id: kbId ? parseInt(kbId) : undefined,
                        use_rag: useRAG
                    })
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                let conversationIdResult = conversationId;
                let contextResult = '';

                while (true) {
                    const { done, value } = await reader.read();
                    
                    if (done) {
                        break;
                    }

                    buffer += decoder.decode(value, { stream: true });
                    
                    // 处理SSE数据
                    const lines = buffer.split('\n');
                    buffer = lines.pop(); // 保留不完整的行

                    for (const line of lines) {
                        if (line.startsWith('data: ')) {
                            try {
                                const data = JSON.parse(line.slice(6));
                                
                                switch (data.type) {
                                    case 'start':
                                        if (data.data.conversation_id) {
                                            conversationIdResult = data.data.conversation_id;
                                        }
                                        break;
                                    case 'context':
                                        if (data.data.documents && onContext) {
                                            onContext(data.data.documents);
                                        }
                                        if (data.data.context) {
                                            contextResult = data.data.context;
                                        }
                                        break;
                                    case 'content':
                                        if (onMessage) {
                                            onMessage(data.data.content);
                                        }
                                        break;
                                    case 'end':
                                        if (onComplete) {
                                            onComplete({
                                                conversation_id: data.data.conversation_id || conversationIdResult,
                                                context: contextResult,
                                                timestamp: data.data.timestamp
                                            });
                                        }
                                        return;
                                    case 'error':
                                        if (onError) {
                                            onError(new Error(data.data.message || 'Stream error'));
                                        }
                                        return;
                                }
                            } catch (e) {
                                console.error('Failed to parse SSE data:', line, e);
                            }
                        }
                    }
                }
            } catch (error) {
                if (onError) {
                    onError(error);
                }
            }
        },

        async listConversations(page = 1, pageSize = 10) {
            return await api.request(`/chat/conversations?page=${page}&page_size=${pageSize}`);
        },

        async getConversation(id) {
            return await api.request(`/chat/conversations/${id}`);
        }
    },

    // 系统相关
    system: {
        async getStats() {
            return await api.request('/system/stats');
        },

        async getConfig() {
            return await api.request('/system/config');
        },

        async updateConfig(configs) {
            return await api.request('/system/config', {
                method: 'PUT',
                body: JSON.stringify({ configs })
            });
        }
    },

    // 用户管理相关
    users: {
        async list(page = 1, pageSize = 10) {
            return await api.request(`/users?page=${page}&page_size=${pageSize}`);
        },

        async get(id) {
            return await api.request(`/users/${id}`);
        },

        async create(userData) {
            return await api.request('/users', {
                method: 'POST',
                body: JSON.stringify(userData)
            });
        },

        async update(id, userData) {
            return await api.request(`/users/${id}`, {
                method: 'PUT',
                body: JSON.stringify(userData)
            });
        },

        async delete(id) {
            return await api.request(`/users/${id}`, {
                method: 'DELETE'
            });
        },

        async updateStatus(id, status) {
            return await api.request(`/users/${id}/status`, {
                method: 'PUT',
                body: JSON.stringify({ status })
            });
        }
    }
};

// 导出到全局
window.utils = utils;
window.api = api;
