import 'dart:io';

import 'package:dio/dio.dart';
import 'package:tiktok_clone/features/messaging/data/models/message_model.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

// ---------------------------------------------------------------------------
// Abstract contract
// ---------------------------------------------------------------------------

abstract class MessageRemoteDataSource {
  /// GET /conversations?cursor=...
  Future<(List<ConversationModel>, String?)> getConversations({
    String? cursor,
  });

  /// GET /messages/:convId?cursor=...
  Future<(List<MessageModel>, String?)> getMessages(
    String conversationId, {
    String? cursor,
  });

  /// POST /messages/:convId
  Future<MessageModel> sendMessage(
    String conversationId,
    String content,
    MessageType type, {
    String? replyToId,
    String? mediaUrl,
  });

  /// POST /messages/media  (multipart)
  Future<String> uploadMedia(File file);

  /// PUT /conversations/:convId/read
  Future<void> markRead(String conversationId);

  /// POST /conversations/group
  Future<ConversationModel> createGroup(List<String> userIds, String name);
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

class MessageRemoteDataSourceImpl implements MessageRemoteDataSource {
  final Dio _dio;

  const MessageRemoteDataSourceImpl(this._dio);

  @override
  Future<(List<ConversationModel>, String?)> getConversations({
    String? cursor,
  }) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/conversations',
      queryParameters: {
        if (cursor != null) 'cursor': cursor,
      },
    );
    final data = response.data!;
    final items = (data['data'] as List<dynamic>)
        .map((e) => ConversationModel.fromJson(e as Map<String, dynamic>))
        .toList();
    final nextCursor = data['next_cursor'] as String?;
    return (items, nextCursor);
  }

  @override
  Future<(List<MessageModel>, String?)> getMessages(
    String conversationId, {
    String? cursor,
  }) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/messages',
      queryParameters: {
        'conversation_id': conversationId,
        if (cursor != null) 'cursor': cursor,
      },
    );
    final data = response.data!;
    final items = (data['data'] as List<dynamic>)
        .map((e) => MessageModel.fromJson(e as Map<String, dynamic>))
        .toList();
    final nextCursor = data['next_cursor'] as String?;
    return (items, nextCursor);
  }

  @override
  Future<MessageModel> sendMessage(
    String conversationId,
    String content,
    MessageType type, {
    String? replyToId,
    String? mediaUrl,
  }) async {
    final response = await _dio.post<Map<String, dynamic>>(
      '/messages',
      data: {
        'conversation_id': conversationId,
        'content': content,
        'type': type.name,
        if (replyToId != null) 'reply_to_id': replyToId,
        if (mediaUrl != null) 'media_url': mediaUrl,
      },
    );
    return MessageModel.fromJson(response.data!);
  }

  @override
  Future<String> uploadMedia(File file) async {
    final formData = FormData.fromMap({
      'file': await MultipartFile.fromFile(
        file.path,
        filename: file.path.split(Platform.pathSeparator).last,
      ),
    });
    final response = await _dio.post<Map<String, dynamic>>(
      '/media/upload',
      data: formData,
      options: Options(
        headers: {'Content-Type': 'multipart/form-data'},
      ),
    );
    return response.data!['media_url'] as String;
  }

  @override
  Future<void> markRead(String conversationId) async {
    await _dio.post<void>(
      '/messages/read',
      data: {'conversation_id': conversationId},
    );
  }

  @override
  Future<ConversationModel> createGroup(
    List<String> userIds,
    String name,
  ) async {
    final response = await _dio.post<Map<String, dynamic>>(
      '/groups',
      data: {
        'user_ids': userIds,
        'name': name,
      },
    );
    return ConversationModel.fromJson(response.data!);
  }
}
