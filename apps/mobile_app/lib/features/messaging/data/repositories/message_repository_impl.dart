import 'dart:io';

import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/features/messaging/data/datasources/message_remote_datasource.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/repositories/message_repository.dart';

class MessageRepositoryImpl implements MessageRepository {
  final MessageRemoteDataSource _remote;

  const MessageRepositoryImpl(this._remote);

  @override
  Future<Either<String, (List<ConversationEntity>, String?)>>
      getConversations({String? cursor}) async {
    try {
      final (models, nextCursor) =
          await _remote.getConversations(cursor: cursor);
      final entities = models.map((m) => m.toEntity()).toList();
      return Right((entities, nextCursor));
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  @override
  Future<Either<String, (List<MessageEntity>, String?)>> getMessages(
    String conversationId, {
    String? cursor,
  }) async {
    try {
      final (models, nextCursor) =
          await _remote.getMessages(conversationId, cursor: cursor);
      final entities = models.map((m) => m.toEntity()).toList();
      return Right((entities, nextCursor));
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  @override
  Future<Either<String, MessageEntity>> sendMessage(
    String conversationId,
    String content,
    MessageType type, {
    String? replyToId,
    String? mediaUrl,
  }) async {
    try {
      final model = await _remote.sendMessage(
        conversationId,
        content,
        type,
        replyToId: replyToId,
        mediaUrl: mediaUrl,
      );
      return Right(model.toEntity());
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  @override
  Future<Either<String, String>> uploadMedia(File file) async {
    try {
      final url = await _remote.uploadMedia(file);
      return Right(url);
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  @override
  Future<Either<String, Unit>> markRead(String conversationId) async {
    try {
      await _remote.markRead(conversationId);
      return const Right(unit);
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  @override
  Future<Either<String, ConversationEntity>> createGroup(
    List<String> userIds,
    String name,
  ) async {
    try {
      final model = await _remote.createGroup(userIds, name);
      return Right(model.toEntity());
    } catch (e) {
      return Left(_errorMessage(e));
    }
  }

  String _errorMessage(Object e) {
    if (e is Exception) return e.toString().replaceFirst('Exception: ', '');
    return e.toString();
  }
}
