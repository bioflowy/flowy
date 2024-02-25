type ParsedURI = {
    scheme: string;
    userinfo: string | undefined;
    host: string | undefined;
    port: string | undefined;
    path: string;
    query: string | undefined;
    fragment: string | undefined;
    reference: string | undefined;
  };
  type URIComponents = {
    scheme?: string;
    userinfo?: string;
    host?: string;
    port?: number | string;
    path?: string;
    query?: string | undefined;
    fragment?: string;
  };
  
export  function serialize(components: URIComponents): string {
    const {
      scheme = '',
      userinfo = '',
      host = '',
      port = '',
      path = '',
      query = {},
      fragment = ''
    } = components;
  
    let uri = '';
  
    // スキームを追加
    if (scheme) {
      uri += `${scheme}:`;
    }
  
    // オーソリティ（ユーザー情報、ホスト、ポート）を追加
    if (host || userinfo) {
      uri += '//';
      if (userinfo) {
        uri += `${userinfo}@`;
      }
      uri += host;
      if (port) {
        uri += `:${port}`;
      }
    }
  
    // パスを追加
    uri += path;
  
    // クエリを追加
    if (query) {
      const searchParams = new URLSearchParams(query as any).toString();
      uri += `?${searchParams}`;
    }
  
    // フラグメントを追加
    if (fragment) {
      uri += `#${fragment}`;
    }
  
    return uri;
  }
export function parse(uri: string, base?: string): ParsedURI {
    let url: URL;
    
    try {
      url = new URL(uri, base);
    } catch (e) {
      const [path, query = '', fragment = ''] = uri.split(/[\?#]/);
      return {
        scheme: '',
        userinfo: undefined,
        host: undefined,
        port: undefined,
        path,
        query: query ? query.split('#')[0] : undefined, // クエリがあれば#以前を取得
        fragment: fragment || undefined, // フラグメントがあれば取得
        reference: 'relative',
      };
    }
  
    // ユーザー情報を抽出
    const userinfo = url.username ? `${url.username}${url.password ? `:${url.password}` : ''}` : undefined;
  
    // ポートが標準の場合は無視
    const port = url.port ? url.port : undefined;
  
    // クエリ文字列の先頭の?を削除
    const query = url.search ? url.search.substring(1) : undefined;
  
    // フラグメントの先頭の#を削除
    const fragment = url.hash ? url.hash.substring(1) : undefined;
  
    return {
      scheme: url.protocol ? url.protocol.replace(':', '') : '', // コロンを削除
      userinfo,
      host: url.hostname || undefined, // 空文字の場合はundefinedに変換
      port,
      path: url.pathname,
      query,
      fragment,
      reference: 'absolute',
    };
  }