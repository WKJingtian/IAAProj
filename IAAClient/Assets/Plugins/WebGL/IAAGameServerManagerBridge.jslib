mergeInto(LibraryManager.library, {
  IAA_RequestFriendsStateData: function () {
    var globalObj = typeof globalThis !== "undefined" ? globalThis : window;
    if (!globalObj) {
      return;
    }

    var manager = null;
    if (
      globalObj.GameServerManager &&
      typeof globalObj.GameServerManager.getFriendsStateData === "function"
    ) {
      manager = globalObj.GameServerManager;
    } else if (
      globalObj.gameServerManager &&
      typeof globalObj.gameServerManager.getFriendsStateData === "function"
    ) {
      manager = globalObj.gameServerManager;
    } else if (
      globalObj.wx &&
      typeof globalObj.wx.getGameServerManager === "function"
    ) {
      try {
        var wxManager = globalObj.wx.getGameServerManager();
        if (wxManager && typeof wxManager.getFriendsStateData === "function") {
          manager = wxManager;
        }
      } catch (error) {
        console.warn(
          "[GameManagerBridge] wx.getGameServerManager failed:",
          error
        );
      }
    }

    if (!manager) {
      console.warn(
        "[GameManagerBridge] GameServerManager.getFriendsStateData not found."
      );
      return;
    }

    try {
      var option = {
        success: function (res) {
          console.log(
            "[GameManagerBridge] getFriendsStateData success:",
            res || {}
          );
        },
        fail: function (err) {
          console.warn(
            "[GameManagerBridge] getFriendsStateData failed:",
            err || {}
          );
        },
        complete: function (res) {
          console.log(
            "[GameManagerBridge] getFriendsStateData complete:",
            res || {}
          );
        }
      };
      var result = manager.getFriendsStateData(option);
      if (result && typeof result.then === "function") {
        result
          .then(function (res) {
            console.log(
              "[GameManagerBridge] getFriendsStateData promise success:",
              res || {}
            );
          })
          .catch(function (err) {
            console.warn(
              "[GameManagerBridge] getFriendsStateData promise failed:",
              err || {}
            );
          });
      }
    } catch (error) {
      console.error("[GameManagerBridge] getFriendsStateData exception:", error);
    }
  },
  IAA_RequestFriendsStateDataWithCallback: function (gameObjectNamePtr) {
    var gameObjectName = UTF8ToString(gameObjectNamePtr);
    var globalObj = typeof globalThis !== "undefined" ? globalThis : window;
    var manager = null;
    var completed = false;

    function serialize(value) {
      try {
        return JSON.stringify(value || {});
      } catch (error) {
        return String(value);
      }
    }

    function succeed(res) {
      if (completed) {
        return;
      }

      completed = true;
      SendMessage(gameObjectName, "OnFriendsStateDataSuccess", serialize(res));
    }

    function fail(err) {
      if (completed) {
        return;
      }

      completed = true;
      SendMessage(gameObjectName, "OnFriendsStateDataFailed", serialize(err));
    }

    if (
      globalObj &&
      globalObj.GameServerManager &&
      typeof globalObj.GameServerManager.getFriendsStateData === "function"
    ) {
      manager = globalObj.GameServerManager;
    } else if (
      globalObj &&
      globalObj.gameServerManager &&
      typeof globalObj.gameServerManager.getFriendsStateData === "function"
    ) {
      manager = globalObj.gameServerManager;
    } else if (
      globalObj &&
      globalObj.wx &&
      typeof globalObj.wx.getGameServerManager === "function"
    ) {
      try {
        manager = globalObj.wx.getGameServerManager();
      } catch (error) {
        fail(error);
        return;
      }
    }

    if (!manager || typeof manager.getFriendsStateData !== "function") {
      fail({ errMsg: "getFriendsStateData not found" });
      return;
    }

    try {
      var result = manager.getFriendsStateData({
        success: succeed,
        fail: fail
      });

      if (result && typeof result.then === "function") {
        result.then(succeed).catch(fail);
      }
    } catch (error) {
      fail(error);
    }
  }
});
