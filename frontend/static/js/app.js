class RAGChatApp {
    constructor() {
        this.documents = [];
        this.eventSource = null;
        this.isStreaming = false;
        
        this.initElements();
        this.initEventListeners();
        this.checkSystemStatus();
        this.loadDocuments();
    }
    
    initElements() {
        // 上传相关
        this.uploadArea = document.getElementById('uploadArea');
        this.fileInput = document.getElementById('fileInput');
        this.uploadProgress = document.getElementById('uploadProgress');
        this.documentsList = document.getElementById('documentsList');
        
        // 聊天相关
        this.chatMessages = document.getElementById('chatMessages');
        this.messageInput = document.getElementById('messageInput');
        this.sendButton = document.getElementById('sendButton');
        this.clearChat = document.getElementById('clearChat');
        this.exportChat = document.getElementById('exportChat');
        
        // 选项相关
        this.showContext = document.getElementById('showContext');
        this.topK = document.getElementById('topK');
        this.topKValue = document.getElementById('topKValue');
        
        // 状态相关
        this.connectionStatus = document.getElementById('connectionStatus');
        this.docCount = document.getElementById('docCount');
        this.vectorDim = document.getElementById('vectorDim');
    }
    
    initEventListeners() {
        // 文件上传 - 由于CSS已经让input覆盖整个uploadArea，所以不需要手动触发click
        this.fileInput.addEventListener('change', (e) => this.handleFileSelect(e.target.files));
        
        // 拖拽上传
        this.uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            this.uploadArea.classList.add('dragging');
        });
        
        this.uploadArea.addEventListener('dragleave', (e) => {
            // 只有当离开uploadArea本身时才移除样式，避免子元素触发
            if (!this.uploadArea.contains(e.relatedTarget)) {
                this.uploadArea.classList.remove('dragging');
            }
        });
        
        this.uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            this.uploadArea.classList.remove('dragging');
            this.handleFileSelect(e.dataTransfer.files);
        });
        
        // 聊天输入
        this.sendButton.addEventListener('click', () => this.sendMessage());
        this.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });
        
        // 自动调整输入框高度
        this.messageInput.addEventListener('input', () => {
            this.messageInput.style.height = 'auto';
            this.messageInput.style.height = this.messageInput.scrollHeight + 'px';
        });
        
        // 快速操作按钮
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('quick-action')) {
                this.messageInput.value = e.target.dataset.query;
                this.sendMessage();
            }
        });
        
        // TopK 滑块
        this.topK.addEventListener('input', (e) => {
            this.topKValue.textContent = e.target.value;
        });
        
        // 清空和导出
        this.clearChat.addEventListener('click', () => this.clearChatHistory());
        this.exportChat.addEventListener('click', () => this.exportChatHistory());
    }
    
    async handleFileSelect(files) {
        for (const file of files) {
            await this.uploadFile(file);
        }
    }
    
    async uploadFile(file) {
        const formData = new FormData();
        formData.append('file', file);
        
        // 显示进度条
        this.uploadProgress.classList.remove('hidden');
        const progressFill = this.uploadProgress.querySelector('.progress-fill');
        const progressText = this.uploadProgress.querySelector('.progress-text');
        
        let uploadComplete = false; // 防止重复通知
        
        try {
            const xhr = new XMLHttpRequest();
            
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    const percentComplete = Math.round((e.loaded / e.total) * 100);
                    progressFill.style.width = percentComplete + '%';
                    progressText.textContent = percentComplete + '%';
                }
            });
            
            xhr.addEventListener('load', () => {
                if (uploadComplete) return; // 防止重复处理
                uploadComplete = true;
                
                if (xhr.status === 200) {
                    try {
                        const response = JSON.parse(xhr.responseText);
                        if (response.success) {
                            this.showNotification('文件上传成功', 'success');
                            this.loadDocuments();
                        } else {
                            this.showNotification(`上传失败: ${response.message || '未知错误'}`, 'error');
                        }
                    } catch (e) {
                        this.showNotification('文件上传失败: 响应解析错误', 'error');
                    }
                } else {
                    this.showNotification(`文件上传失败: HTTP ${xhr.status}`, 'error');
                }
                
                setTimeout(() => {
                    this.uploadProgress.classList.add('hidden');
                    progressFill.style.width = '0%';
                    progressText.textContent = '0%';
                }, 1000);
            });
            
            xhr.addEventListener('error', () => {
                if (uploadComplete) return; // 防止重复处理
                uploadComplete = true;
                
                this.showNotification('网络错误，文件上传失败', 'error');
                this.uploadProgress.classList.add('hidden');
            });
            
            xhr.open('POST', '/api/v1/upload');
            xhr.send(formData);
            
        } catch (error) {
            if (uploadComplete) return; // 防止重复处理
            uploadComplete = true;
            
            console.error('Upload error:', error);
            this.showNotification('文件上传失败', 'error');
            this.uploadProgress.classList.add('hidden');
        }
    }
    
    async sendMessage() {
        const message = this.messageInput.value.trim();
        if (!message || this.isStreaming) return;
        
        // 添加用户消息
        this.addMessage(message, 'user');
        this.messageInput.value = '';
        this.messageInput.style.height = 'auto';
        
        // 添加加载指示器
        const loadingId = this.addLoadingIndicator();
        
        // 开始流式响应
        this.isStreaming = true;
        this.sendButton.disabled = true;
        
        try {
            // 使用 SSE 接收流式响应
            const params = new URLSearchParams({
                query: message,
                top_k: this.topK.value,
                return_context: this.showContext.checked
            });
            
            this.eventSource = new EventSource(`/api/v1/chat/stream?${params}`);
            
            let assistantMessage = '';
            let messageElement = null;
            
            this.eventSource.onmessage = (event) => {
                const data = JSON.parse(event.data);
                
                if (data.type === 'start') {
                    // 移除加载指示器
                    document.getElementById(loadingId)?.remove();
                    // 创建助手消息元素
                    messageElement = this.addMessage('', 'assistant', false);
                } else if (data.type === 'content') {
                    // 更新消息内容
                    assistantMessage += data.data.content;
                    messageElement.querySelector('.message-text').textContent = assistantMessage;
                    this.scrollToBottom();
                } else if (data.type === 'context' && this.showContext.checked) {
                    // 显示检索上下文
                    this.addContextToMessage(messageElement, data.data.documents);
                } else if (data.type === 'end') {
                    // 结束流式响应
                    this.eventSource.close();
                    this.isStreaming = false;
                    this.sendButton.disabled = false;
                } else if (data.type === 'error') {
                    // 处理错误
                    this.showNotification(data.data.message, 'error');
                    this.eventSource.close();
                    this.isStreaming = false;
                    this.sendButton.disabled = false;
                    document.getElementById(loadingId)?.remove();
                }
            };
            
            this.eventSource.onerror = (error) => {
                console.error('SSE error:', error);
                this.eventSource.close();
                this.isStreaming = false;
                this.sendButton.disabled = false;
                document.getElementById(loadingId)?.remove();
                this.showNotification('连接错误，请重试', 'error');
            };
            
        } catch (error) {
            console.error('Send message error:', error);
            this.isStreaming = false;
            this.sendButton.disabled = false;
            document.getElementById(loadingId)?.remove();
            this.showNotification('发送失败，请重试', 'error');
        }
    }
    
    addMessage(content, sender, scroll = true) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message message-${sender}`;
        
        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';
        
        const textDiv = document.createElement('div');
        textDiv.className = 'message-text';
        textDiv.textContent = content;
        
        const timeDiv = document.createElement('div');
        timeDiv.className = 'message-time';
        timeDiv.textContent = new Date().toLocaleTimeString();
        
        contentDiv.appendChild(textDiv);
        contentDiv.appendChild(timeDiv);
        messageDiv.appendChild(contentDiv);
        
        // 移除欢迎消息
        const welcomeMsg = this.chatMessages.querySelector('.welcome-message');
        if (welcomeMsg) {
            welcomeMsg.remove();
        }
        
        this.chatMessages.appendChild(messageDiv);
        
        if (scroll) {
            this.scrollToBottom();
        }
        
        return messageDiv;
    }
    
    addLoadingIndicator() {
        const loadingId = 'loading-' + Date.now();
        const loadingDiv = document.createElement('div');
        loadingDiv.id = loadingId;
        loadingDiv.className = 'message message-assistant';
        loadingDiv.innerHTML = `
            <div class="message-content">
                <div class="typing-indicator">
                    <span class="typing-dot"></span>
                    <span class="typing-dot"></span>
                    <span class="typing-dot"></span>
                </div>
            </div>
        `;
        
        this.chatMessages.appendChild(loadingDiv);
        this.scrollToBottom();
        
        return loadingId;
    }
    
    addContextToMessage(messageElement, documents) {
        if (!documents || documents.length === 0) return;
        
        const contextDiv = document.createElement('div');
        contextDiv.className = 'context-section';
        
        const headerDiv = document.createElement('div');
        headerDiv.className = 'context-header';
        headerDiv.textContent = `检索到 ${documents.length} 个相关文档片段:`;
        contextDiv.appendChild(headerDiv);
        
        documents.forEach((doc, index) => {
            const itemDiv = document.createElement('div');
            itemDiv.className = 'context-item';
            
            const scoreSpan = document.createElement('span');
            scoreSpan.className = 'context-score';
            scoreSpan.textContent = `[相关度: ${doc.score.toFixed(3)}] `;
            
            const contentSpan = document.createElement('span');
            contentSpan.textContent = doc.content.substring(0, 200) + '...';
            
            itemDiv.appendChild(scoreSpan);
            itemDiv.appendChild(contentSpan);
            contextDiv.appendChild(itemDiv);
        });
        
        messageElement.querySelector('.message-content').appendChild(contextDiv);
    }
    
    async loadDocuments() {
        try {
            const response = await fetch('/api/v1/documents');
            const data = await response.json();
            
            if (data.success && data.documents) {
                this.documents = data.documents;
                this.updateDocumentsList();
                this.docCount.textContent = data.count || '0';
            }
        } catch (error) {
            console.error('Load documents error:', error);
        }
    }
    
    updateDocumentsList() {
        if (this.documents.length === 0) {
            this.documentsList.innerHTML = '<div class="empty-state">暂无文档</div>';
            return;
        }
        
        this.documentsList.innerHTML = '';
        this.documents.forEach(doc => {
            const docDiv = document.createElement('div');
            docDiv.className = 'document-item';
            
            const nameDiv = document.createElement('div');
            nameDiv.className = 'document-name';
            nameDiv.textContent = doc.filename || '未命名文档';
            
            const sizeDiv = document.createElement('div');
            sizeDiv.className = 'document-size';
            sizeDiv.textContent = this.formatFileSize(doc.size || 0);
            
            docDiv.appendChild(nameDiv);
            docDiv.appendChild(sizeDiv);
            this.documentsList.appendChild(docDiv);
        });
    }
    
    async checkSystemStatus() {
        try {
            const response = await fetch('/api/v1/health');
            const data = await response.json();
            
            if (response.ok) {
                this.connectionStatus.textContent = '在线';
                this.connectionStatus.className = 'status-value status-online';
            } else {
                this.connectionStatus.textContent = '离线';
                this.connectionStatus.className = 'status-value status-offline';
            }
        } catch (error) {
            this.connectionStatus.textContent = '离线';
            this.connectionStatus.className = 'status-value status-offline';
        }
    }
    
    clearChatHistory() {
        if (confirm('确定要清空对话历史吗？')) {
            this.chatMessages.innerHTML = `
                <div class="welcome-message">
                    <h2>👋 欢迎使用 RAG 聊天系统</h2>
                    <p>上传文档后，我可以基于文档内容回答您的问题</p>
                    <div class="quick-actions">
                        <button class="quick-action" data-query="系统是如何工作的？">系统是如何工作的？</button>
                        <button class="quick-action" data-query="支持哪些文件格式？">支持哪些文件格式？</button>
                        <button class="quick-action" data-query="如何提高搜索准确度？">如何提高搜索准确度？</button>
                    </div>
                </div>
            `;
        }
    }
    
    exportChatHistory() {
        const messages = this.chatMessages.querySelectorAll('.message');
        let exportText = '=== RAG Chat Export ===\n\n';
        
        messages.forEach(msg => {
            const sender = msg.classList.contains('message-user') ? 'User' : 'Assistant';
            const text = msg.querySelector('.message-text').textContent;
            const time = msg.querySelector('.message-time').textContent;
            exportText += `[${time}] ${sender}: ${text}\n\n`;
        });
        
        const blob = new Blob([exportText], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `chat-export-${Date.now()}.txt`;
        a.click();
        URL.revokeObjectURL(url);
    }
    
    scrollToBottom() {
        this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
    }
    
    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }
    
    showNotification(message, type = 'info') {
        console.log(`[${type}] ${message}`);
        
        // 创建toast通知元素
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.textContent = message;
        
        // 添加到页面
        document.body.appendChild(toast);
        
        // 显示动画
        setTimeout(() => {
            toast.classList.add('show');
        }, 10);
        
        // 自动消失
        setTimeout(() => {
            toast.classList.remove('show');
            setTimeout(() => {
                if (toast.parentNode) {
                    document.body.removeChild(toast);
                }
            }, 300);
        }, 3000);
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    new RAGChatApp();
});
