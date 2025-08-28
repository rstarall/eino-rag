// 页面初始化
document.addEventListener('DOMContentLoaded', async () => {
    // 获取当前路径
    const currentPath = window.location.pathname;
    console.log('Current path:', currentPath);
    
    // 定义不需要认证的页面
    const publicPages = ['/login', '/register'];
    
    // 如果是公开页面，检查是否已登录
    if (publicPages.includes(currentPath)) {
        const token = utils.getToken();
        if (token) {
            // 有token，尝试验证
            try {
                const profileResult = await api.auth.getProfile();
                if (profileResult.success && profileResult.user) {
                    const user = profileResult.user;
                    utils.setUser(user);
                    // 已登录，根据角色重定向
                    if (user.role_name === 'admin') {
                        window.location.href = '/admin/dashboard';
                    } else {
                        window.location.href = '/';
                    }
                    return;
                }
            } catch (error) {
                // Token无效，清除
                utils.clearToken();
                utils.clearUser();
            }
        }
        // 继续显示公开页面
        return;
    }
    
    // 所有其他页面都需要认证
    const token = utils.getToken();
    if (!token) {
        console.log('No token, redirecting to login');
        window.location.href = '/login';
        return;
    }
    
    // 验证token
    try {
        const profileResult = await api.auth.getProfile();
        if (!profileResult.success || !profileResult.user) {
            throw new Error('Invalid token');
        }
        
        const user = profileResult.user;
        utils.setUser(user);
        
        // 如果访问admin页面，检查权限
        if (currentPath.startsWith('/admin')) {
            if (user.role_name !== 'admin') {
                utils.showMessage('您没有管理员权限', 'error');
                window.location.href = '/';
                return;
            }
        }
        
        // Token有效，用户有权限，继续显示页面
    } catch (error) {
        console.log('Token validation failed:', error);
        utils.clearToken();
        utils.clearUser();
        window.location.href = '/login';
    }

    // 设置用户信息
    const user = utils.getUser();
    if (user) {
        const userNameElements = document.querySelectorAll('.user-name');
        userNameElements.forEach(el => {
            el.textContent = user.name;
        });
    }

    // 绑定登出按钮
    const logoutBtns = document.querySelectorAll('.logout-btn');
    logoutBtns.forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.preventDefault();
            try {
                await api.auth.logout();
            } catch (error) {
                utils.showMessage('退出登录失败', 'error');
            }
        });
    });
});