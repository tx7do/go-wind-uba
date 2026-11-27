/// GoWind UBA Flutter SDK —— 风行用户行为分析平台埋点采集。
///
/// 面向 Android / iOS / Windows / macOS / Linux，直连 collector 上报协议
/// （`POST /uba/v1/report`，鉴权在请求 body，camelCase JSON）。
///
/// ```dart
/// import 'package:uba_flutter/uba_flutter.dart';
///
/// void main() async {
///   WidgetsFlutterBinding.ensureInitialized();
///   final client = await UbaClient.init(UbaConfig(
///     appId: 'your_app_id',
///     appSecret: 'your_app_secret',
///     endpoint: 'http://collector:5700',
///   ));
///   UbaLifecycleObserver(client).register();
///   client.track('button_click', properties: {'scene': 'home'});
/// }
/// ```
library;

export 'src/batcher.dart';
export 'src/client.dart';
export 'src/config.dart';
export 'src/context.dart';
export 'src/flags.dart';
export 'src/lifecycle.dart';
export 'src/session.dart';
export 'src/store.dart';
export 'src/transport.dart';
export 'src/types.dart';
