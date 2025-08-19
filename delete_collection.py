#!/usr/bin/env python3
"""
删除指定的Milvus集合的脚本
"""
import os
from pymilvus import connections, Collection, utility

def main():
    # 从环境变量读取配置
    host = os.getenv('MILVUS_HOST', 'localhost')
    port = int(os.getenv('MILVUS_PORT', '19530'))
    collection_name = os.getenv('COLLECTION_NAME', 'rag_documents_bge_m3')
    
    print(f"连接到 Milvus: {host}:{port}")
    print(f"目标集合: {collection_name}")
    
    try:
        # 连接到Milvus
        connections.connect(
            alias="default",
            host=host,
            port=port
        )
        
        # 检查集合是否存在
        if utility.has_collection(collection_name):
            print(f"集合 {collection_name} 存在，正在删除...")
            
            # 删除集合
            utility.drop_collection(collection_name)
            print(f"✅ 集合 {collection_name} 已成功删除")
        else:
            print(f"❌ 集合 {collection_name} 不存在")
            
        # 列出所有集合
        collections = utility.list_collections()
        print(f"📋 当前存在的集合: {collections}")
        
    except Exception as e:
        print(f"❌ 错误: {e}")
    finally:
        connections.disconnect("default")
        print("🔌 已断开连接")

if __name__ == "__main__":
    main()
